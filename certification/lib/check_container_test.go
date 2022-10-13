package lib

import (
	"context"
	"os"
	"testing"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/formatters"
	"github.com/stretchr/testify/assert"
)

func TestCheckContainer(t *testing.T) {
	// TODO: Preflight has various functions that reach back into artifacts
	// and writes files to the artifacts directory. CheckContainer doesn't technically
	// set a value for that so it's just writing in CWD. Would potentially need to figure
	// out how to access artifacts in a library implementation. Afero would probably
	// have some ƒunctionality that would work here.
	image := "quay.io/opdev/simple-demo-operator:0.0.6"
	r, e := CheckContainer(image)
	assert.NoError(t, e)
	fmt, _ := formatters.NewByName("json")
	rb, _ := fmt.Format(context.TODO(), r)
	os.WriteFile("results", rb, 0o755)
}
