package lib

import (
	"context"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/engine"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/formatters"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/policy"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/runtime"
)

// CheckContainer runs preflight's container checks on i.
func CheckContainer(i string) (runtime.Results, error) {
	ctx := context.TODO()
	c := runtime.Config{
		Image:          i,
		Policy:         policy.PolicyContainer,
		ResponseFormat: formatters.DefaultFormat,
	}

	e, err := engine.NewForConfig(ctx, c.ReadOnly())
	if err != nil {
		return runtime.Results{}, err
	}

	err = e.ExecuteChecks(ctx)
	if err != nil {
		return runtime.Results{}, err
	}

	return e.Results(ctx), nil
}
