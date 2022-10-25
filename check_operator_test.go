package preflight_test

// import (
// 	"context"
// 	"testing"

// 	preflight "github.com/redhat-openshift-ecosystem/openshift-preflight"
// 	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/formatters"
// 	"github.com/stretchr/testify/assert"
// )

// func TestCheckOperator(t *testing.T) {
// 	kubeconfig := "/path/to/some/kubeconfig"
// 	image := "quay.io/opdev/simple-demo-operator-bundle:latest"
// 	catalog := "quay.io/opdev/simple-demo-operator-catalog:latest"

// 	chk := preflight.NewOperatorCheck(image, kubeconfig, catalog)
// 	results, err := chk.Run(context.TODO())
// 	assert.NoError(t, err, "should not throw an error")

// 	f, _ := formatters.NewByName("json")
// 	b, _ := f.Format(context.TODO(), results)

// 	t.Log(string(b))
// }
