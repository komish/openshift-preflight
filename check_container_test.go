package preflight_test

import (
	"context"
	"testing"

	preflight "github.com/redhat-openshift-ecosystem/openshift-preflight"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/formatters"
	"github.com/stretchr/testify/assert"
)

func TestContainerCheck(t *testing.T) {
	chk := preflight.NewContainerCheck("quay.io/opdev/simple-demo-operator:latest")
	results, artf, err := chk.Run(context.TODO())
	assert.NoError(t, err, "should not throw an error")

	f, _ := formatters.NewByName("json")
	b, _ := f.Format(context.TODO(), results)

	t.Log("artifacts:", artf)
	t.Log("results", string(b))
}
