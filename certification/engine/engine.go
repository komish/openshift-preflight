// Package engine contains the interfaces necessary to implement policy execution.
package engine

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification"
	internal "github.com/redhat-openshift-ecosystem/openshift-preflight/certification/internal/engine"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/internal/operatorsdk"
	containerpol "github.com/redhat-openshift-ecosystem/openshift-preflight/certification/internal/policy/container"
	operatorpol "github.com/redhat-openshift-ecosystem/openshift-preflight/certification/internal/policy/operator"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/policy"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/pyxis"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/runtime"
)

// CheckEngine defines the functionality necessary to run all checks for a policy,
// and return the results of that check execution.
type CheckEngine interface {
	// ExecuteChecks should execute all checks in a policy and internally
	// store the results. Errors returned by ExecuteChecks should reflect
	// errors in pre-validation tasks, and not errors in individual check
	// execution itself.
	ExecuteChecks(context.Context) error
	// Results returns the outcome of executing all checks.
	Results(context.Context) runtime.Results
}

func New(ctx context.Context,
	image string,
	isBundle,
	isScratch bool,
	// TODO(JOSE): The problem with accepting checks as a parameter is that that allows consumers
	// to build with this engine arbitrary checks. This may not be a bad thing, but is something to
	// consider.
	checks []certification.Check,
	dockerconfig string,
) (CheckEngine, error) {
	return &internal.CraneEngine{
		DockerConfig: dockerconfig,
		Image:        image,
		Checks:       checks,
		IsBundle:     isBundle,
		IsScratch:    isScratch,
	}, nil
}

func NewForConfig(ctx context.Context, cfg certification.Config) (CheckEngine, error) {
	checks, err := InitializeChecks(ctx, cfg.Policy(), cfg)
	if err != nil {
		return nil, fmt.Errorf("error initializing checks: %v", err)
	}

	return New(
		ctx,
		cfg.Image(),
		cfg.IsBundle(),
		cfg.IsScratch(),
		checks,
		cfg.DockerConfig(),
	)
}

// OperatorCheckConfig contains configuration relevant to an individual check's execution.
// TODO(JOSE): move this to the right module, this probably doesn't need to exist here
type OperatorCheckConfig struct {
	ScorecardImage, ScorecardWaitTime, Namespace, ServiceAccount string
	IndexImage, DockerConfig, Channel, Kubeconfig                string
}

// InitializeOperatorChecks returns checks for policy p give cfg.
func InitializeOperatorChecks(ctx context.Context, p policy.Policy, cfg OperatorCheckConfig) ([]certification.Check, error) {
	switch p {
	case policy.PolicyOperator:
		return []certification.Check{
			operatorpol.NewScorecardBasicSpecCheck(operatorsdk.New(cfg.ScorecardImage, exec.Command), cfg.Namespace, cfg.ServiceAccount, cfg.Kubeconfig, cfg.ScorecardWaitTime),
			operatorpol.NewScorecardOlmSuiteCheck(operatorsdk.New(cfg.ScorecardImage, exec.Command), cfg.Namespace, cfg.ServiceAccount, cfg.Kubeconfig, cfg.ScorecardWaitTime),
			operatorpol.NewDeployableByOlmCheck(cfg.IndexImage, cfg.DockerConfig, cfg.Channel),
			operatorpol.NewValidateOperatorBundleCheck(),
			operatorpol.NewCertifiedImagesCheck(pyxis.NewPyxisClient(
				certification.DefaultPyxisHost,
				"",
				"",
				&http.Client{Timeout: 60 * time.Second}),
			),
			operatorpol.NewSecurityContextConstraintsCheck(),
			&operatorpol.RelatedImagesCheck{},
		}, nil
	}

	return nil, fmt.Errorf("provided policy %s is unknown", p)
}

// ContainerCheckConfig contains configuration relevant to an individual check's execution.
type ContainerCheckConfig struct {
	DockerConfig, PyxisAPIToken, CertificationProjectID string
}

// InitializeContainerChecks returns the appropriate checks for policy p given cfg.
func InitializeContainerChecks(ctx context.Context, p policy.Policy, cfg ContainerCheckConfig) ([]certification.Check, error) {
	switch p {
	case policy.PolicyContainer:
		return []certification.Check{
			&containerpol.HasLicenseCheck{},
			containerpol.NewHasUniqueTagCheck(cfg.DockerConfig),
			&containerpol.MaxLayersCheck{},
			&containerpol.HasNoProhibitedPackagesCheck{},
			&containerpol.HasRequiredLabelsCheck{},
			&containerpol.RunAsNonRootCheck{},
			&containerpol.HasModifiedFilesCheck{},
			containerpol.NewBasedOnUbiCheck(pyxis.NewPyxisClient(
				certification.DefaultPyxisHost,
				cfg.PyxisAPIToken,
				cfg.CertificationProjectID,
				&http.Client{Timeout: 60 * time.Second})),
		}, nil
	case policy.PolicyRoot:
		return []certification.Check{
			&containerpol.HasLicenseCheck{},
			containerpol.NewHasUniqueTagCheck(cfg.DockerConfig),
			&containerpol.MaxLayersCheck{},
			&containerpol.HasNoProhibitedPackagesCheck{},
			&containerpol.HasRequiredLabelsCheck{},
			&containerpol.HasModifiedFilesCheck{},
			containerpol.NewBasedOnUbiCheck(pyxis.NewPyxisClient(
				certification.DefaultPyxisHost,
				cfg.PyxisAPIToken,
				cfg.CertificationProjectID,
				&http.Client{Timeout: 60 * time.Second})),
		}, nil
	case policy.PolicyScratch:
		return []certification.Check{
			&containerpol.HasLicenseCheck{},
			containerpol.NewHasUniqueTagCheck(cfg.DockerConfig),
			&containerpol.MaxLayersCheck{},
			&containerpol.HasRequiredLabelsCheck{},
			&containerpol.RunAsNonRootCheck{},
		}, nil
	}

	return nil, fmt.Errorf("provided policy %s is unknown", p)
}

// InitializeChecks configures checks for a given policy p using cfg as needed.
// // TODO(JOSE): Remove this function if we continue to use the standalone container/operator ones.
//
//nolint:unparam // ctx is unused. Keep for future use.
func InitializeChecks(ctx context.Context, p policy.Policy, cfg certification.Config) ([]certification.Check, error) {
	switch p {
	case policy.PolicyOperator:
		return []certification.Check{
			operatorpol.NewScorecardBasicSpecCheck(operatorsdk.New(cfg.ScorecardImage(), exec.Command), cfg.Namespace(), cfg.ServiceAccount(), cfg.Kubeconfig(), cfg.ScorecardWaitTime()),
			operatorpol.NewScorecardOlmSuiteCheck(operatorsdk.New(cfg.ScorecardImage(), exec.Command), cfg.Namespace(), cfg.ServiceAccount(), cfg.Kubeconfig(), cfg.ScorecardWaitTime()),
			operatorpol.NewDeployableByOlmCheck(cfg.IndexImage(), cfg.DockerConfig(), cfg.Channel()),
			operatorpol.NewValidateOperatorBundleCheck(),
			operatorpol.NewCertifiedImagesCheck(pyxis.NewPyxisClient(
				certification.DefaultPyxisHost,
				"",
				"",
				&http.Client{Timeout: 60 * time.Second}),
			),
			operatorpol.NewSecurityContextConstraintsCheck(),
			&operatorpol.RelatedImagesCheck{},
		}, nil
	case policy.PolicyContainer:
		return []certification.Check{
			&containerpol.HasLicenseCheck{},
			containerpol.NewHasUniqueTagCheck(cfg.DockerConfig()),
			&containerpol.MaxLayersCheck{},
			&containerpol.HasNoProhibitedPackagesCheck{},
			&containerpol.HasRequiredLabelsCheck{},
			&containerpol.RunAsNonRootCheck{},
			&containerpol.HasModifiedFilesCheck{},
			containerpol.NewBasedOnUbiCheck(pyxis.NewPyxisClient(
				certification.DefaultPyxisHost,
				cfg.PyxisAPIToken(),
				cfg.CertificationProjectID(),
				&http.Client{Timeout: 60 * time.Second})),
		}, nil
	case policy.PolicyRoot:
		return []certification.Check{
			&containerpol.HasLicenseCheck{},
			containerpol.NewHasUniqueTagCheck(cfg.DockerConfig()),
			&containerpol.MaxLayersCheck{},
			&containerpol.HasNoProhibitedPackagesCheck{},
			&containerpol.HasRequiredLabelsCheck{},
			&containerpol.HasModifiedFilesCheck{},
			containerpol.NewBasedOnUbiCheck(pyxis.NewPyxisClient(
				certification.DefaultPyxisHost,
				cfg.PyxisAPIToken(),
				cfg.CertificationProjectID(),
				&http.Client{Timeout: 60 * time.Second})),
		}, nil
	case policy.PolicyScratch:
		return []certification.Check{
			&containerpol.HasLicenseCheck{},
			containerpol.NewHasUniqueTagCheck(cfg.DockerConfig()),
			&containerpol.MaxLayersCheck{},
			&containerpol.HasRequiredLabelsCheck{},
			&containerpol.RunAsNonRootCheck{},
		}, nil
	}

	return nil, fmt.Errorf("provided policy %s is unknown", p)
}

// makeCheckList returns a list of check names.
func makeCheckList(checks []certification.Check) []string {
	checkNames := make([]string, len(checks))

	for i, check := range checks {
		checkNames[i] = check.Name()
	}

	return checkNames
}

// checkNamesFor produces a slice of names for checks in the requested policy.
func checkNamesFor(ctx context.Context, p policy.Policy) []string {
	// stub the config. We don't technically need the policy here, but why not.
	c := &runtime.Config{Policy: p}
	checks, _ := InitializeChecks(ctx, p, c.ReadOnly())
	return makeCheckList(checks)
}

// OperatorPolicy returns the names of checks in the operator policy.
func OperatorPolicy(ctx context.Context) []string {
	return checkNamesFor(ctx, policy.PolicyOperator)
}

// ContainerPolicy returns the names of checks in the container policy.
func ContainerPolicy(ctx context.Context) []string {
	return checkNamesFor(ctx, policy.PolicyContainer)
}

// ScratchContainerPolicy returns the names of checks in the
// container policy with scratch exception.
func ScratchContainerPolicy(ctx context.Context) []string {
	return checkNamesFor(ctx, policy.PolicyScratch)
}

// RootExceptionContainerPolicy returns the names of checks in the
// container policy with root exception.
func RootExceptionContainerPolicy(ctx context.Context) []string {
	return checkNamesFor(ctx, policy.PolicyRoot)
}
