package cmd

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/artifacts"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/pyxis"
	log "github.com/sirupsen/logrus"
)

// noopSubmitter is a no-op resultSubmitter that optionally logs a message
// and a reason as to why results were not submitted.
type noopSubmitter struct {
	emitLog bool
	reason  string
	log     *log.Logger
}

func (s *noopSubmitter) Submit(ctx context.Context) error {
	if s.emitLog {
		msg := "Results are not being sent for submission."
		if s.reason != "" {
			msg = fmt.Sprintf("%s Reason: %s.", msg, s.reason)
		}

		s.log.Info(msg)
	}

	return nil
}

// containerCertificationSubmitter submits container results to Pyxis, and implements
// a resultSubmitter.
type containerCertificationSubmitter struct {
	certificationProjectID string
	pyxis                  pyxisClient
	dockerConfig           string
	preflightLogFile       string
}

func (s *containerCertificationSubmitter) Submit(ctx context.Context) error {
	log.Info("preparing results that will be submitted to Red Hat")

	// get the project info from pyxis
	certProject, err := s.pyxis.GetProject(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve project: %w", err)
	}

	// Ensure that a certProject was returned. In theory we would expect pyxis
	// to throw an error if no project is returned, but in the event that it doesn't
	// we need to confirm before we proceed in order to prevent a runtime panic
	// setting the DockerConfigJSON below.
	if certProject == nil {
		return fmt.Errorf("no certification project was returned from pyxis")
	}

	log.Tracef("CertProject: %+v", certProject)

	// read the provided docker config
	dockerConfigJsonBytes, err := os.ReadFile(s.dockerConfig)
	if err != nil {
		return err
	}

	certProject.Container.DockerConfigJSON = string(dockerConfigJsonBytes)

	// prepare submission. We ignore the error because nil checks for the certProject
	// are done earlier to prevent panics, and that's the only error case for this function.
	submission, _ := pyxis.NewCertificationInput(certProject)

	certImage, err := os.Open(path.Join(artifacts.Path(), certification.DefaultCertImageFilename))
	defer certImage.Close()
	if err != nil {
		return fmt.Errorf("could not open file for submission: %s: %w",
			certification.DefaultCertImageFilename,
			err,
		)
	}
	preflightResults, err := os.Open(path.Join(artifacts.Path(), certification.DefaultTestResultsFilename))
	defer preflightResults.Close()
	if err != nil {
		return fmt.Errorf(
			"could not open file for submission: %s: %w",
			certification.DefaultTestResultsFilename,
			err,
		)
	}
	rpmManifest, err := os.Open(path.Join(artifacts.Path(), certification.DefaultRPMManifestFilename))
	defer rpmManifest.Close()
	if err != nil {
		return fmt.Errorf(
			"could not open file for submission: %s: %w",
			certification.DefaultRPMManifestFilename,
			err,
		)
	}
	logfile, err := os.Open(s.preflightLogFile)
	defer logfile.Close()
	if err != nil {
		return fmt.Errorf(
			"could not open file for submission: %s: %w",
			s.preflightLogFile,
			err,
		)
	}
	submission.
		// The engine writes the certified image config to disk in a Pyxis-specific format.
		WithCertImage(certImage).
		// Include Preflight's test results in our submission. pyxis.TestResults embeds them.
		WithPreflightResults(preflightResults).
		// The certification engine writes the rpmManifest for images not based on scratch.
		WithRPMManifest(rpmManifest).
		// Include the preflight execution log file.
		WithArtifact(logfile, filepath.Base(s.preflightLogFile))

	input, err := submission.Finalize()
	if err != nil {
		return fmt.Errorf("unable to finalize data that would be sent to pyxis: %w", err)
	}

	certResults, err := s.pyxis.SubmitResults(ctx, input)
	if err != nil {
		return fmt.Errorf("could not submit to pyxis: %w", err)
	}

	log.Info("Test results have been submitted to Red Hat.")
	log.Info("These results will be reviewed by Red Hat for final certification.")
	log.Infof("The container's image id is: %s.", certResults.CertImage.ID)
	log.Infof("Please check %s to view scan results.", buildScanResultsURL(s.certificationProjectID, certResults.CertImage.ID))
	log.Infof("Please check %s to monitor the progress.", buildOverviewURL(s.certificationProjectID))

	return nil
}
