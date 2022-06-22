package cmd

import (
	"context"
	"io"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/pyxis"
)

type resultWriter interface {
	OpenFile(name string) (io.WriteCloser, error)
	io.WriteCloser
}

type resultSubmitter interface {
	Submit(context.Context) error
}

// pyxisClient defines pyxis API interactions that are relevant to check executions in cmd.
type pyxisClient interface {
	FindImagesByDigest(ctx context.Context, digests []string) ([]pyxis.CertImage, error)
	GetProject(context.Context) (*pyxis.CertProject, error)
	SubmitResults(context.Context, *pyxis.CertificationInput) (*pyxis.CertificationResults, error)
}
