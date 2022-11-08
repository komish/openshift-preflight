package lib

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLib(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "lib Suite")
}

// var createAndCleanupDirForArtifactsAndLogs = func() context.Context {
// 	tmpDir, err := os.MkdirTemp("", "lib-execute-*")
// 	Expect(err).ToNot(HaveOccurred())
// 	artifacts.SetDir(filepath.Join(tmpDir, "artifacts"))
// 	os.Setenv("PFLT_ARTIFACTS", artifacts.Path())
// 	os.Setenv("PFLT_LOGFILE", filepath.Join(tmpDir, "preflight.log"))
// 	DeferCleanup(os.RemoveAll, tmpDir)
// 	DeferCleanup(os.Unsetenv, "PFLT_ARTIFACTS")
// 	DeferCleanup(os.Unsetenv, "PFLT_LOGFILE")
// }
