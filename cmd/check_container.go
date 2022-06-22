package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/artifacts"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/engine"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/formatters"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/policy"
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
	Args:  checkContainerPositionalArgs,
	// this fmt.Sprintf is in place to keep spacing consistent with cobras two spaces that's used in: Usage, Flags, etc
	Example: fmt.Sprintf("  %s", "preflight check container quay.io/repo-name/container-name:version"),
	RunE:    checkContainerRunE,
}

// checkContainerRunE executes checkContainer using the user args to inform the execution.
func checkContainerRunE(cmd *cobra.Command, args []string) error {
	log.Info("certification library version ", version.Version.String())
	ctx := cmd.Context()
	containerImage := args[0]

	// Render the Viper configuration as a runtime.Config
	cfg, err := runtime.NewConfigFrom(*viper.GetViper())
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Set our runtime defaults.
	cfg.Image = containerImage
	cfg.ResponseFormat = formatters.DefaultFormat
	cfg.Policy = policy.PolicyContainer
	cfg.Submit = submit

	pyxisClient := newPyxisClient(ctx, cfg.ReadOnly()) // (JOSE) this can return nil and that is valid, so we need to make sure we always check for nil when working with pyxisclient

	// If we have a pyxisClient, we can query for container policy exceptions.
	if pyxisClient != nil {
		policy, err := getContainerPolicyExceptions(ctx, pyxisClient)
		if err != nil {
			return err
		}

		cfg.Policy = policy
	}

	engine, err := engine.NewForConfig(ctx, cfg.ReadOnly())
	if err != nil {
		return err
	}

	fmttr, err := formatters.NewForConfig(cfg.ReadOnly())
	if err != nil {
		return err
	}

	rs := resolveSubmitter(pyxisClient, cfg.ReadOnly())

	// Run the  container check.
	cmd.SilenceUsage = true

	return newCheckContainerFunc(ctx,
		cfg,
		pyxisClient,
		engine,
		fmttr,
		&runtime.ResultWriterFile{},
		rs,
	)
}

// (JOSE) cmd package namespace is getting full, start using either an internal package, moving things to another package, or something else.

// resolveSubmitter will build out a resultSubmitter if the provided pyxisClient, pc, is not nil.
// The pyxisClient is a required component of the submitter. If pc is nil, then a noop submitter
// is returned instead, which does nothing.
func resolveSubmitter(pc pyxisClient, cfg certification.Config) resultSubmitter {
	if pc != nil {
		return &containerCertificationSubmitter{
			certificationProjectID: cfg.CertificationProjectID(),
			pyxis:                  pc,
			dockerConfig:           cfg.DockerConfig(),
			preflightLogFile:       cfg.LogFile(), // (JOSE) make sure this actually works... is this the full path to the file? what's being used in other places.
		}
	}

	return &noopSubmitter{emitLog: true}
}

// newPyxisClient initializes a pyxis.PyxisClient with relevant information from cfg.
// If the the CertificationProjectID, PyxisAPIToken, or PyxisHost are empty, then nil is returned.
// Callers should treat a nil pyxis client as an indicator that pyxis calls should not be made.
func newPyxisClient(ctx context.Context, cfg certification.Config) pyxisClient {
	if cfg.CertificationProjectID() == "" || cfg.PyxisAPIToken() == "" || cfg.PyxisHost() == "" {
		return nil
	}

	return pyxis.NewPyxisClient(
		cfg.PyxisHost(),
		cfg.PyxisAPIToken(),
		cfg.CertificationProjectID(),
		&http.Client{Timeout: 60 * time.Second},
	)
}

// getContainerPolicyExceptions will query Pyxis to determine if
// a given project has a certification excemptions, such as root or scratch.
// This will then return the corresponding policy.
//
// If no policy exception flags are found on the project, the standard
// container policy is returned.
func getContainerPolicyExceptions(ctx context.Context, pc pyxisClient) (policy.Policy, error) {
	certProject, err := pc.GetProject(ctx)
	if err != nil {
		return "", fmt.Errorf("could not retrieve project: %w", err)
	}
	log.Debugf("Certification project name is: %s", certProject.Name)
	if certProject.Container.OsContentType == "scratch" {
		return policy.PolicyScratch, nil
	}

	// if a partner sets `Host Level Access` in connect to `Privileged`, enable RootExceptionContainerPolicy checks
	if certProject.Container.Privileged {
		return policy.PolicyRoot, nil
	}
	return policy.PolicyContainer, nil
}

// newCheckContainerfunc executes checks, interacts with pyxis, format output, writes, and submits results.
func newCheckContainerFunc(
	ctx context.Context,
	cfg *runtime.Config,
	pc pyxisClient,
	eng engine.CheckEngine,
	formatter formatters.ResponseFormatter,
	rw resultWriter,
	rs resultSubmitter,
) error {
	// configure the artifacts directory if the user requested a different directory.
	if cfg.Artifacts != "" {
		artifacts.SetDir(cfg.Artifacts)
	}

	// create the results file early to catch cases where we are not
	// able to write to the filesystem before we attempt to execute checks.
	resultsFile, err := rw.OpenFile(filepath.Join(artifacts.Path(), resultsFilenameWithExtension(formatter.FileExtension())))
	if err != nil {
		return err
	}
	defer resultsFile.Close()

	resultsOutputTarget := io.MultiWriter(os.Stdout, resultsFile)

	// execute the checks
	if err := eng.ExecuteChecks(ctx); err != nil {
		return err
	}
	results := eng.Results(ctx)

	// return results to the user and then close output files
	formattedResults, err := formatter.Format(ctx, results)
	if err != nil {
		return err
	}

	fmt.Fprintln(resultsOutputTarget, string(formattedResults))

	if cfg.WriteJUnit {
		if err := writeJUnit(ctx, results); err != nil {
			return err
		}
	}

	if cfg.Submit {
		if err := rs.Submit(ctx); err != nil {
			return err
		}
	}

	log.Infof("Preflight result: %s", convertPassedOverall(results.PassedOverall))

	return nil
}

func checkContainerPositionalArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("a container image positional argument is required")
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
