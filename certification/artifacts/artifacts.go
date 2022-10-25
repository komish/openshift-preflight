// Package artifacts provides functionality for writing artifact files in configured
// artifacts directory. This package operators with a singleton directory variable that can be
// changed and reset. It provides simple functionality that can be accessible from
// any calling library.
package artifacts

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// appFS is the base path FS to base writes on
var appFS = afero.NewOsFs()

// ads is the artifacts directory singleton.
var ads string

// DefaultArtifactsDir is the default value for the directory.
const DefaultArtifactsDir = "artifacts"

func init() {
	Reset()
}

// SetDir sets the package level artifacts directory. This
// can be a relative path or a full path.
func SetDir(s string) {
	fullPath := s
	if !strings.HasPrefix(s, "/") {
		cwd, _ := os.Getwd()
		fullPath = filepath.Join(cwd, s)
	}
	ads = fullPath
}

// Reset restores the default value for the Artifacts Directory.
func Reset() {
	// set the singleton to the default value.
	cwd, _ := os.Getwd()
	ads = filepath.Join(cwd, DefaultArtifactsDir)
}

// WriteFile will write contents of the string to a file in
// the artifacts directory. It will create the artifacts dir
// if necessary.
// Returns the full path (including the artifacts dir)
func WriteFile(filename string, contents io.Reader) (string, error) {
	fullFilePath := filepath.Join(Path(), filename)

	if err := afero.SafeWriteReader(appFS, fullFilePath, contents); err != nil {
		return fullFilePath, fmt.Errorf("could not write file to artifacts directory: %v", err)
	}
	return fullFilePath, nil
}

// Path will return the artifacts directory.
func Path() string {
	return ads
}

// ContextWithWriter adds ArtifactWriter w to the context ctx.
func ContextWithWriter(ctx context.Context, w ArtifactWriter) context.Context {
	return context.WithValue(ctx, contextKey(), w)
}

// WriterFromContext returns the writer from the context, or nil.
func WriterFromContext(ctx context.Context) ArtifactWriter {
	w := ctx.Value(contextKey())
	if writer, ok := w.(ArtifactWriter); ok {
		return writer
	}

	return nil // TODO(Jose): Return a noop ArtifactWriter?
}

// artifactsWriterContextKey is a key used to store/retrieve ArtifactsWriter in/from context.Context.
type artifactsWriterContextKey string

// contextKey returns the context key for an Artifacts Writer.
func contextKey() artifactsWriterContextKey {
	return artifactsWriterContextKey("ArtifactWriter")
}

// ArtifactWriter describes functionality required for writing artifacts.
// TODO(Jose): We technically shouldn't need this here because this package should
// contain implementations, and the library package should contain the interface.
//
// Move this where it should be after the PoC has been demonstrated.
type ArtifactWriter interface {
	WriteFile(filename string, contents io.Reader) (string, error)
}

// resolveFullPath resolves the full path of s if s is a relative path.
func resolveFullPath(s string) string {
	fullPath := s
	if !strings.HasPrefix(s, "/") {
		cwd, _ := os.Getwd()
		fullPath = filepath.Join(cwd, s)
	}
	return fullPath
}
