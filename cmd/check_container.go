package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/artifacts"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/engine"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/formatters"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/pyxis"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/runtime"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/version"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var submit bool

var checkContainerCmd = &cobra.Command{
	Use:   "container",
	Short: "Run checks for a container",
	Long:  `This command will run the Certification checks for a container image. `,
	Args: func(cmd *cobra.Command, args []string) error {
		if l, _ := cmd.Flags().GetBool("list-checks"); l {
			fmt.Printf("\n%s\n%s%s\n", "The checks that will be executed are the following:", "- ",
				strings.Join(engine.ContainerPolicy(), "\n- "))

			// exiting gracefully instead of retuning, otherwise cobra calls RunE
			os.Exit(0)
		}

		if len(args) != 1 {
			return fmt.Errorf("%w: A container image positional argument is required", ErrInsufficientPosArguments)
		}

		if submit {
			if !viper.IsSet("certification_project_id") {
				cmd.MarkFlagRequired("certification-project-id")
			}

			if !viper.IsSet("pyxis_api_token") {
				cmd.MarkFlagRequired("pyxis-api-token")
			}
		}

		return nil
	},
	// this fmt.Sprintf is in place to keep spacing consistent with cobras two spaces that's used in: Usage, Flags, etc
	Example: fmt.Sprintf("  %s", "preflight check container quay.io/repo-name/container-name:version"),
	PreRun:  preRunConfig,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Info("certification library version ", version.Version.String())
		ctx := context.Background()
		containerImage := args[0]

		// Render the Viper configuration as a runtime.Config
		cfg, err := runtime.NewConfigFrom(*viper.GetViper())
		if err != nil {
			return err // TODO(Jose) Log/Handle Error
		}

		// Set our runtime defaults.
		cfg.Image = containerImage
		cfg.ResponseFormat = DefaultOutputFormat
		cfg.EnabledChecks = engine.ContainerPolicy()

		// Determine if we need to modify the policy that's executed.
		if cfg.CertificationProjectID != "" {
			pyxisClient := pyxis.NewPyxisClient(
				cfg.PyxisHost,
				cfg.PyxisAPIToken,
				cfg.CertificationProjectID,
				&http.Client{Timeout: 60 * time.Second},
			)
			certProject, err := pyxisClient.GetProject(ctx)
			if err != nil {
				log.Error(fmt.Errorf("%w: %s", ErrRetrievingProject, err))
				return err
			}
			log.Debugf("Certification project name is: %s", certProject.Name)
			if certProject.Container.OsContentType == "scratch" {
				cfg.EnabledChecks = engine.ScratchContainerPolicy()
				cfg.Scratch = true
			}

			// if a partner sets `Host Level Access` in connect to `Privileged`, enable RootExceptionContainerPolicy checks
			if certProject.Container.Privileged {
				cfg.EnabledChecks = engine.RootExceptionContainerPolicy()
			}
		}

		// Run the  container check.
		return CheckContainer(ctx, *cfg)
	},
}

// CheckContainer runs the Container policy.
func CheckContainer(ctx context.Context, cfg runtime.Config) error {
	engine, err := engine.NewForConfig(cfg)
	if err != nil {
		return err
	}

	formatter, err := formatters.NewForConfig(cfg)
	if err != nil {
		return err
	}

	// create the results file early to catch cases where we are not
	// able to write to the filesystem before we attempt to execute checks.
	resultsFile, err := os.OpenFile(
		filepath.Join(artifacts.Path(), resultsFilenameWithExtension(formatter.FileExtension())),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		0o600,
	)
	if err != nil {
		return err
	}

	// also write to stdout
	resultsOutputTarget := io.MultiWriter(os.Stdout, resultsFile)

	// At this point, we would no longer want usage information printed out
	// on error, so it doesn't contaminate the output.
	// NOTE: We lose the ability to silence usage in this extracted function.
	// cmd.SilenceUsage = true

	// execute the checks
	if err := engine.ExecuteChecks(ctx); err != nil {
		return err
	}
	results := engine.Results(ctx)

	// return results to the user and then close output files
	formattedResults, err := formatter.Format(ctx, results)
	if err != nil {
		return err
	}

	fmt.Fprintln(resultsOutputTarget, string(formattedResults))
	if err := resultsFile.Close(); err != nil {
		return err
	}

	if err := writeJunitIfEnabled(ctx, results); err != nil {
		return err
	}

	// assemble artifacts and submit results to pyxis if user provided the submit flag.
	if submit {
		log.Info("preparing results that will be submitted to Red Hat")

		// you must provide a project ID in order to submit.
		if cfg.CertificationProjectID == "" {
			return ErrEmptyProjectID
		}

		// establish a pyxis client.
		pyxisClient := pyxis.NewPyxisClient(
			cfg.PyxisHost,
			cfg.PyxisAPIToken,
			cfg.CertificationProjectID,
			&http.Client{Timeout: 60 * time.Second})

		// get the project info from pyxis
		certProject, err := pyxisClient.GetProject(ctx)
		if err != nil {
			log.Error(fmt.Errorf("%w: %s", ErrRetrievingProject, err))
			return err
		}
		log.Tracef("CertProject: %+v", certProject)

		// read the provided docker config
		dockerConfigJsonBytes, err := os.ReadFile(cfg.DockerConfig)
		if err != nil {
			return err
		}

		certProject.Container.DockerConfigJSON = string(dockerConfigJsonBytes)

		// prepare submission
		submission, err := pyxis.NewCertificationInput(certProject)
		if err != nil {
			return fmt.Errorf(
				"%w: could not build submission with required assets: %s",
				ErrSubmittingToPyxis,
				err,
			)
		}

		certImage, err := os.Open(path.Join(artifacts.Path(), certification.DefaultCertImageFilename))
		defer certImage.Close()
		if err != nil {
			return fmt.Errorf(
				"%w: could not open file for submission: %s",
				err,
				certification.DefaultCertImageFilename,
			)
		}
		preflightResults, err := os.Open(path.Join(artifacts.Path(), certification.DefaultTestResultsFilename))
		defer preflightResults.Close()
		if err != nil {
			return fmt.Errorf(
				"%w: could not open file for submission: %s",
				err,
				certification.DefaultTestResultsFilename,
			)
		}
		rpmManifest, err := os.Open(path.Join(artifacts.Path(), certification.DefaultRPMManifestFilename))
		defer rpmManifest.Close()
		if err != nil {
			return fmt.Errorf(
				"%w: could not open file for submission: %s",
				err,
				certification.DefaultRPMManifestFilename,
			)
		}
		logfile, err := os.Open(viper.GetString("logfile"))
		defer logfile.Close()
		if err != nil {
			return fmt.Errorf(
				"%w: could not open file for submission: %s",
				err,
				cfg.LogFile,
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
			WithArtifact(logfile, filepath.Base(cfg.LogFile))

		input, err := submission.Finalize()
		if err != nil {
			return fmt.Errorf(
				"%w: unable to finalize data that would be sent to pyxis %s",
				ErrSubmittingToPyxis,
				err,
			)
		}

		certResults, err := pyxisClient.SubmitResults(ctx, input)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrSubmittingToPyxis, err)
		}

		log.Info("Test results have been submitted to Red Hat.")
		log.Info("These results will be reviewed by Red Hat for final certification.")
		log.Infof("The container's image id is: %s.", certResults.CertImage.ID)
		log.Infof("Please check %s to view scan results.", buildScanResultsURL(cfg.CertificationProjectID, certResults.CertImage.ID))
		log.Infof("Please check %s to monitor the progress.", buildOverviewURL(cfg.CertificationProjectID))
	}

	log.Infof("Preflight result: %s", convertPassedOverall(results.PassedOverall))

	return nil
}

func init() {
	checkContainerCmd.Flags().BoolVarP(&submit, "submit", "s", false, "submit check container results to red hat")
	viper.BindPFlag("submit", checkContainerCmd.Flags().Lookup("submit"))

	checkContainerCmd.Flags().String("pyxis-api-token", "", "API token for Pyxis authentication (env: PFLT_PYXIS_API_TOKEN)")
	viper.BindPFlag("pyxis_api_token", checkContainerCmd.Flags().Lookup("pyxis-api-token"))

	checkContainerCmd.Flags().String("pyxis-host", "", fmt.Sprintf("Host to use for Pyxis submissions. This will override Pyxis Env. Only set this if you know what you are doing.\n"+
		"If you do set it, it should include just the host, and the URI path. (env: PFLT_PYXIS_HOST)"))
	viper.BindPFlag("pyxis_host", checkContainerCmd.Flags().Lookup("pyxis-host"))

	checkContainerCmd.Flags().String("pyxis-env", certification.DefaultPyxisEnv, "Env to use for Pyxis submissions.")
	viper.BindPFlag("pyxis_env", checkContainerCmd.Flags().Lookup("pyxis-env"))

	checkContainerCmd.Flags().String("certification-project-id", "", fmt.Sprintf("Certification Project ID from connect.redhat.com/projects/{certification-project-id}/overview\n"+
		"URL paramater. This value may differ from the PID on the overview page. (env: PFLT_CERTIFICATION_PROJECT_ID)"))
	viper.BindPFlag("certification_project_id", checkContainerCmd.Flags().Lookup("certification-project-id"))

	checkCmd.AddCommand(checkContainerCmd)
}
