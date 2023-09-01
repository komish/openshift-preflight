// Package types contains all types relevant to this PoC.
//
// This is organized into a single place just for PoC purposes.
// These are copied from preflight because preflight contains these in
// an internal package.
package plugin

import (
	"context"
	"time"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/internal/check"
	libformatters "github.com/redhat-openshift-ecosystem/openshift-preflight/internal/formatters"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/internal/image"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/internal/lib"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/internal/runtime"
)

// Note(komish): This file exports types required to implement a Plugin that are
// currently private/unexported. This includes types previously locked behind
// the internal package, and so those are aliased here until we decide to
// collapse the code down here.

// CheckEngine defines the functionality necessary to run all checks for a policy,
// and return the results of that check execution.
type CheckEngine interface {
	// ExecuteChecks should execute all checks in a policy and internally
	// store the results. Errors returned by ExecuteChecks should reflect
	// errors in pre-validation tasks, and not errors in individual check
	// execution itself.
	ExecuteChecks(context.Context) error
	// Results returns the outcome of executing all checks.
	Results(context.Context) Results
}

// These types just make internal types public for use by plugin developers.
type ImageReference = image.ImageReference

type ResultSubmitter = lib.ResultSubmitter

type ResultWriter = lib.ResultWriter

// Note(komish): Metadata types may be useful across all certificatoins, but
// check.Check makes more sense to exist in cert/operator plugin code itself, or
// a shared library across those two.
type Check = check.Check

type CheckMetadata = check.Metadata

type CheckHelpText = check.HelpText

type OpenshiftClusterVersion = runtime.OpenshiftClusterVersion

// Result is the same as check.Result but doesn't embed the check.Check!
// The intention here is to prevent other cerifications from needing to adhere
// to the same check abstractions used for container and operator certs.
type Result struct {
	Check       CheckInfo
	ElapsedTime time.Duration
}

type CheckInfo struct {
	Name     func() string
	Metadata func() check.Metadata
	Help     func() check.HelpText
}

// This type doesn't make as much sense as we integrate certifications that aren't tied
// explicitly to container images, but I'm leaving it in tact for simplicity.
type Results struct {
	TestedImage       string
	PassedOverall     bool
	TestedOn          OpenshiftClusterVersion
	CertificationHash string
	Passed            []Result
	Failed            []Result
	Errors            []Result
}

// PluginResultFromCertificationResult returns a plugin Result from the certification.Result
// Note(komish): This is a convenience function that can be removed if using this new Result
// is the preferred way to go.
func PluginResultsFromCertificationResults(r ...certification.Result) []Result {
	res := make([]Result, 0, len(r))
	for _, cr := range r {
		res = append(res, Result{
			Check: CheckInfo{
				Name:     cr.Name,
				Metadata: cr.Metadata,
				Help:     cr.Help,
			},
			ElapsedTime: cr.ElapsedTime,
		})
	}
	return res
}

// Note to Preflight Maintainers:
// This Results struct is being defined here because the previous Result
// contained a check.Check, and it's not logical to expect all certification to
// match that signature.
//
// check.Check is specific to how the preflight container/operator checks run
// and shouldn't be imposed on other certifications.

// _Results is an example of what a future returned result could look like.
type _Results struct {
	Plugin        string
	PluginVersion string
	ElapsedTime   time.Duration
	PassedOverall bool
	// Result is expected to contain the plugin's own result types.
	// This will be written to the result file specified by the user.
	// at the format they specified.
	//
	// It might make sense to define an interface here to delegate
	// formatting to the plugin itself. Alternatively, we may need to
	// fallback in cases where we can't marshal to, say, JSON, and dump
	// the raw values, which is a bit of a gap.
	Result any
}

// Used by Pyxis Structs
type UserResponse = libformatters.UserResponse
