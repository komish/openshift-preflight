package preflight

import (
	"context"
	"errors"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/policy"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/runtime"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/internal/lib"
)

var ErrImageEmpty = errors.New("image is empty")

type ContainerCheckOption = func(*containerCheck)

// NewContainerCheck is a check that runs preflight's Container Policy.
func NewContainerCheck(image string, opts ...ContainerCheckOption) *containerCheck {
	c := &containerCheck{
		image: image,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Run executes the check and returns the results.
func (c containerCheck) Run(ctx context.Context) (runtime.Results, error) {
	if c.image == "" {
		return runtime.Results{}, ErrImageEmpty
	}

	cfg := runtime.Config{
		Image:          c.image,
		ResponseFormat: "json", // TODO: if we don't include this, execution fails.
		Policy:         policy.PolicyContainer,
		WriteJUnit:     false,
		Submit:         false,
	}

	runner, err := lib.NewCheckContainerRunner(ctx, &cfg, false)
	if err != nil {
		return runtime.Results{}, err
	}

	if err := runner.Eng.ExecuteChecks(ctx); err != nil {
		return runtime.Results{}, err
	}

	res := runner.Eng.Results(ctx)
	return res, nil
}

type containerCheck struct {
	image string
	// formatter formatters.ResponseFormatter
}
