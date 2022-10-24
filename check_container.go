package preflight

import (
	"context"
	"errors"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/policy"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/runtime"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/internal/lib"
)

var ErrImageEmpty = errors.New("image is empty")

// TODO: Does this need to be exported?
type containerCheck struct {
	ctx   context.Context
	image string
	// formatter formatters.ResponseFormatter
}

type ContainerCheckOption = func(*containerCheck)

func NewContainerCheck(image string, opts ...ContainerCheckOption) *containerCheck {
	c := &containerCheck{
		ctx:   context.Background(),
		image: image,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// TODO: Implement
// TODO: it may make more sense to go ahead and promote runtime.Results to preflight.Results.
func (c containerCheck) Run() (runtime.Results, error) {
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

	runner, err := lib.NewCheckContainerRunner(c.ctx, &cfg, false)
	if err != nil {
		return runtime.Results{}, err // TODO: wrap - this error comes from outside this lib.
	}

	if err := runner.Eng.ExecuteChecks(c.ctx); err != nil {
		return runtime.Results{}, err // TODO: wrap - this error comes from outside this lib.
	}

	res := runner.Eng.Results(c.ctx)
	return res, nil
}

// WithContext adds the provided context to the exection.
func WithContext(ctx context.Context) ContainerCheckOption {
	return func(c *containerCheck) {
		c.ctx = ctx
	}
}
