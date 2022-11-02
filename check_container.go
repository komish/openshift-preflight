package preflight

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/engine"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/policy"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/pyxis"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/runtime"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/internal/lib"
)

var ErrImageEmpty = errors.New("image is empty")

type ContainerCheckOption = func(*containerCheck)

// NewContainerCheck is a check that runs preflight's Container Policy.
func NewContainerCheck(image string, opts ...ContainerCheckOption) *containerCheck {
	c := &containerCheck{
		image:     image,
		pyxisHost: certification.DefaultPyxisHost,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Run executes the check and returns the results.
func (c *containerCheck) Run(ctx context.Context) (runtime.Results, error) {
	// TODO(JOSE): Need to add the new artifacts writing logic here.
	if c.image == "" {
		return runtime.Results{}, ErrImageEmpty
	}

	policy := policy.PolicyContainer

	if c.HasPyxisData() {
		p := pyxis.NewPyxisClient(
			certification.DefaultPyxisHost,
			c.pyxisToken,
			c.certificationProjectID,
			&http.Client{Timeout: 60 * time.Second},
		)

		override, err := lib.GetContainerPolicyExceptions(ctx, p)
		if err != nil {
			return runtime.Results{}, err // TODO(JOSE): WRAP this error in one relevant to this library.
			// TODO(JOSE): the CLI currently falls back to just running the default policy in this case, but the lib cannot.
		}

		policy = override
	}

	checks, err := engine.InitializeContainerChecks(ctx, policy, engine.ContainerCheckConfig{
		DockerConfig:           c.dockerconfigjson,
		PyxisAPIToken:          c.pyxisToken,
		CertificationProjectID: c.certificationProjectID,
	})
	if err != nil {
		return runtime.Results{}, err // TODO(JOSE): Wrap this error for library calls.
	}

	// TODO(JOSE): Does the engine init need to know about scratch or is that unnecessary now that check resolution
	// happens elsewhere?
	eng, err := engine.New(ctx, c.image, false, false, checks, c.dockerconfigjson)
	if err != nil {
		return runtime.Results{}, err
	}

	if err != nil {
		return runtime.Results{}, err // TODO(JOSE): Wrap
	}

	if err := eng.ExecuteChecks(ctx); err != nil {
		return runtime.Results{}, err
	}

	return eng.Results(ctx), nil
}

// TODO(JOSE) Maybe I need to stuff all the container check stuff in the internal
// library and then create aliases here in the top-level library.

// HasPyxisData returns true of the values necessary to make a pyxis
// API call are not empty. This does not check the validity of the input values.
func (c *containerCheck) HasPyxisData() bool {
	return (c.certificationProjectID != "" && c.pyxisToken != "" && c.pyxisHost != "")
}

// TODO(JOSE): Do we need a path or the data? We prefer the data here.
func WithDockerConfigJSON(s string) ContainerCheckOption {
	return func(cc *containerCheck) {
		cc.dockerconfigjson = s
	}
}

// WithCertificationProject adds the project's id and pyxis token to the check
// allowing for the project's metadata to change the certification (if necessary).
// An example might be the Scratch or Privileged flags on a project allowing for
// the corresponding policy to be executed.
func WithCertificationProject(id, token string) ContainerCheckOption {
	return func(cc *containerCheck) {
		cc.pyxisToken = token
		cc.certificationProjectID = id
	}
}

// WithPyxisHost sets the pyxis host for pyxis interactions.
func WithPyxisHost(host string) ContainerCheckOption {
	return func(cc *containerCheck) {
		cc.pyxisHost = host
	}
}

type containerCheck struct {
	image            string
	dockerconfigjson string
	// TODO(JOSE): consider combining these pyxis components.
	certificationProjectID string
	pyxisToken             string
	pyxisHost              string
}
