package formatters

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/internal/x/version"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/x/plugin/v0"
)

// Note(komish): This package is basically a replica of the internal formatters
// package, except it accepts a plugin.Result instead of a certification.Result.
//
// Not all formatters have been ported here.

// GenericJSONFormatter is a FormatterFunc that formats results as JSON
func GenericJSONFormatter(ctx context.Context, r plugin.Results) ([]byte, error) {
	response := getResponse(r)

	responseJSON, err := json.MarshalIndent(response, "", "    ")
	if err != nil {
		e := fmt.Errorf("error formatting results with formatter %s: %w",
			"json",
			err,
		)

		return nil, e
	}

	return responseJSON, nil
}

// getResponse will extract the runtime's results and format it to fit the
// UserResponse definition in a way that can then be formatted.
func getResponse(r plugin.Results) UserResponse {
	passedChecks := make([]checkExecutionInfo, 0, len(r.Passed))
	failedChecks := make([]checkExecutionInfo, 0, len(r.Failed))
	erroredChecks := make([]checkExecutionInfo, 0, len(r.Errors))

	if len(r.Passed) > 0 {
		for _, result := range r.Passed {
			passedChecks = append(passedChecks, checkExecutionInfo{
				Name:        result.Check.Name(),
				ElapsedTime: float64(result.ElapsedTime.Milliseconds()),
				Description: result.Check.Metadata().Description,
			})
		}
	}

	if len(r.Failed) > 0 {
		for _, result := range r.Failed {
			failedChecks = append(failedChecks, checkExecutionInfo{
				Name:             result.Check.Name(),
				ElapsedTime:      float64(result.ElapsedTime.Milliseconds()),
				Description:      result.Check.Metadata().Description,
				Help:             result.Check.Help().Message,
				Suggestion:       result.Check.Help().Suggestion,
				KnowledgeBaseURL: result.Check.Metadata().KnowledgeBaseURL,
				CheckURL:         result.Check.Metadata().CheckURL,
			})
		}
	}

	if len(r.Errors) > 0 {
		for _, result := range r.Errors {
			erroredChecks = append(erroredChecks, checkExecutionInfo{
				Name:        result.Check.Name(),
				ElapsedTime: float64(result.ElapsedTime.Milliseconds()),
				Description: result.Check.Metadata().Description,
				Help:        result.Check.Help().Message,
			})
		}
	}

	response := UserResponse{
		Image:             r.TestedImage,
		Passed:            r.PassedOverall,
		LibraryInfo:       version.Version,
		CertificationHash: r.CertificationHash,
		Results: resultsText{
			Passed: passedChecks,
			Failed: failedChecks,
			Errors: erroredChecks,
		},
	}

	return response
}

// UserResponse is the standard user-facing response.
type UserResponse struct {
	Image             string                 `json:"image" xml:"image"`
	Passed            bool                   `json:"passed" xml:"passed"`
	CertificationHash string                 `json:"certification_hash,omitempty" xml:"certification_hash,omitempty"`
	LibraryInfo       version.VersionContext `json:"test_library" xml:"test_library"`
	Results           resultsText            `json:"results" xml:"results"`
}

// resultsText represents the results of check execution against the asset.
type resultsText struct {
	Passed []checkExecutionInfo `json:"passed" xml:"passed"`
	Failed []checkExecutionInfo `json:"failed" xml:"failed"`
	Errors []checkExecutionInfo `json:"errors" xml:"errors"`
}

// checkExecutionInfo contains all possible output fields that a user might see in their result.
// Empty fields will be omitted.
type checkExecutionInfo struct {
	Name             string  `json:"name,omitempty" xml:"name,omitempty"`
	ElapsedTime      float64 `json:"elapsed_time" xml:"elapsed_time"`
	Description      string  `json:"description,omitempty" xml:"description,omitempty"`
	Help             string  `json:"help,omitempty" xml:"help,omitempty"`
	Suggestion       string  `json:"suggestion,omitempty" xml:"suggestion,omitempty"`
	KnowledgeBaseURL string  `json:"knowledgebase_url,omitempty" xml:"knowledgebase_url,omitempty"`
	CheckURL         string  `json:"check_url,omitempty" xml:"check_url,omitempty"`
}
