package lib

import (
	"bytes"
	"context"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/artifacts"
	log "github.com/sirupsen/logrus"
)

// LogThroughArtifactWriterIfSet reconfigures the logger used by Preflight to write to
// the artifact writer if one is configured. This is a workaround for the library
// implementation because existing checks make calls directly to logrus.
func LogThroughArtifactWriterIfSet(ctx context.Context) {
	w := artifacts.WriterFromContext(ctx)

	if w != nil {
		log.SetLevel(log.TraceLevel)
		log.SetFormatter(&log.TextFormatter{})
		b := bytes.NewBufferString("")
		log.SetOutput(b)

		w.WriteFile("preflight.log", b)
	}
}

type contextKey string

var executionEnvIsCLI = contextKey("IsCLI")

func CallerIsCLI(ctx context.Context) bool {
	val := ctx.Value(executionEnvIsCLI)
	switch b := val.(type) {
	case bool:
		return b
	default:
		return false
	}
}

// SetCallerToCLI sets the caller as the CLI. NOTE: This is a temporary
// workaround for internal CLI executions and will be removed at a later
// date.
func SetCallerToCLI(ctx context.Context) context.Context {
	return context.WithValue(ctx, executionEnvIsCLI, true)
}
