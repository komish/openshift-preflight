package container

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/internal/check"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/internal/image"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/internal/log"
)

var _ check.Check = &ContainsKernelObjectsCheck{}

// ContainsKernelObjectsCheck detects the presence of kernel modules (.ko, .ko.xz, .ko.gz)
// in container images.
type ContainsKernelObjectsCheck struct{}

func (p *ContainsKernelObjectsCheck) Validate(ctx context.Context, imgRef image.ImageReference) (bool, error) {
	kernelObjects, err := p.getDataToValidate(ctx, imgRef.ImageFSPath)
	if err != nil {
		//coverage:ignore
		return false, fmt.Errorf("could not scan for kernel objects: %v", err)
	}
	return p.validate(ctx, kernelObjects)
}

func (p *ContainsKernelObjectsCheck) getDataToValidate(_ context.Context, imageFSPath string) ([]string, error) {
	var kernelObjects []string

	err := filepath.WalkDir(imageFSPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			//coverage:ignore
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Check if the file has a kernel object extension
		name := d.Name()
		if strings.HasSuffix(name, ".ko") ||
			strings.HasSuffix(name, ".ko.xz") ||
			strings.HasSuffix(name, ".ko.gz") {
			// Store relative path from image root
			relPath, err := filepath.Rel(imageFSPath, path)
			if err != nil {
				//coverage:ignore
				return err
			}
			kernelObjects = append(kernelObjects, relPath)
		}
		return nil
	})

	if err != nil {
		//coverage:ignore
		return nil, fmt.Errorf("error walking filesystem: %w", err)
	}

	return kernelObjects, nil
}

func (p *ContainsKernelObjectsCheck) validate(ctx context.Context, kernelObjectsPaths []string) (bool, error) {
	logger := logr.FromContextOrDiscard(ctx)

	if len(kernelObjectsPaths) > 0 {
		logger.Info("kernel objects detected in container image",
			"count", len(kernelObjectsPaths))

		// Log each kernel object found
		for _, ko := range kernelObjectsPaths {
			logger.V(log.DBG).Info("kernel object found", "path", ko)
		}

		logger.Info("this container contains kernel modules which may indicate portability or security concerns. This is safe to ignore if kernel modules are required for your use case.")
		return false, nil
	}

	logger.V(log.DBG).Info("no kernel objects detected")
	return true, nil
}

func (p *ContainsKernelObjectsCheck) Name() string {
	return "ContainsKernelObjects"
}

func (p *ContainsKernelObjectsCheck) Metadata() check.Metadata {
	return check.Metadata{
		Description:      "Checks for the presence of kernel modules (.ko, .ko.xz, .ko.gz) in the container image which may indicate portability or security concerns",
		Level:            "optional",
		KnowledgeBaseURL: certDocumentationURL,
		CheckURL:         certDocumentationURL,
	}
}

func (p *ContainsKernelObjectsCheck) Help() check.HelpText {
	return check.HelpText{
		Message:    "Check ContainsKernelObjects detected kernel object in the container image.",
		Suggestion: "Kernel modules are not expected to be present in container images deployed on OpenShift",
	}
}

func (p *ContainsKernelObjectsCheck) RequiredFilePatterns() []string {
	return []string{
		"**/*.ko",
		"**/*.ko.xz",
		"**/*.ko.gz",
	}
}
