package container

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/internal/image"
)

func setupKernelCheckTmpDir() string {
	tmpDir, err := os.MkdirTemp("", "kernel-check-*")
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() {
		os.RemoveAll(tmpDir)
	})
	return tmpDir
}

func createKernelModule(tmpDir, path string, content string) {
	fullPath := filepath.Join(tmpDir, path)
	err := os.MkdirAll(filepath.Dir(fullPath), 0o755)
	Expect(err).ToNot(HaveOccurred())
	err = os.WriteFile(fullPath, []byte(content), 0o644)
	Expect(err).ToNot(HaveOccurred())
}

func createNonKernelFile(tmpDir, path string) {
	fullPath := filepath.Join(tmpDir, path)
	err := os.MkdirAll(filepath.Dir(fullPath), 0o755)
	Expect(err).ToNot(HaveOccurred())
	err = os.WriteFile(fullPath, []byte("not a kernel module"), 0o644)
	Expect(err).ToNot(HaveOccurred())
}

var _ = Describe("ContainsKernelObjects", func() {
	var containsKernelObjects ContainsKernelObjectsCheck

	Describe("Checking if kernel objects are found", func() {
		Context("When .ko files are found", func() {
			It("Should not pass Validate", func() {
				tmpDir := setupKernelCheckTmpDir()
				createKernelModule(tmpDir, "driver.ko", "fake kernel module")

				ok, err := containsKernelObjects.Validate(context.TODO(), image.ImageReference{ImageFSPath: tmpDir})
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())
			})
		})

		Context("When .ko.xz files are found", func() {
			It("Should not pass Validate", func() {
				tmpDir := setupKernelCheckTmpDir()
				createKernelModule(tmpDir, "driver.ko.xz", "fake compressed kernel module")

				ok, err := containsKernelObjects.Validate(context.TODO(), image.ImageReference{ImageFSPath: tmpDir})
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())
			})
		})

		Context("When .ko.gz files are found", func() {
			It("Should not pass Validate", func() {
				tmpDir := setupKernelCheckTmpDir()
				createKernelModule(tmpDir, "driver.ko.gz", "fake gzip compressed kernel module")

				ok, err := containsKernelObjects.Validate(context.TODO(), image.ImageReference{ImageFSPath: tmpDir})
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())
			})
		})

		Context("When kernel objects are in nested directories", func() {
			It("Should not pass Validate", func() {
				tmpDir := setupKernelCheckTmpDir()
				createKernelModule(tmpDir, filepath.Join("lib", "modules", "5.14.0", "kernel", "drivers", "e1000.ko.xz"), "fake driver module")

				ok, err := containsKernelObjects.Validate(context.TODO(), image.ImageReference{ImageFSPath: tmpDir})
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())
			})
		})

		Context("When multiple kernel objects exist", func() {
			It("Should not pass Validate", func() {
				tmpDir := setupKernelCheckTmpDir()
				createKernelModule(tmpDir, "driver1.ko", "fake module 1")
				createKernelModule(tmpDir, "driver2.ko.xz", "fake module 2")
				createKernelModule(tmpDir, "driver3.ko.gz", "fake module 3")

				ok, err := containsKernelObjects.Validate(context.TODO(), image.ImageReference{ImageFSPath: tmpDir})
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())
			})
		})

		Context("When kernel objects are not found", func() {
			It("Should pass Validate for empty directory", func() {
				tmpDir := setupKernelCheckTmpDir()

				ok, err := containsKernelObjects.Validate(context.TODO(), image.ImageReference{ImageFSPath: tmpDir})
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})
		})

		Context("When only non-kernel files exist", func() {
			It("Should pass Validate", func() {
				tmpDir := setupKernelCheckTmpDir()
				createNonKernelFile(tmpDir, "app.so")
				createNonKernelFile(tmpDir, "config.ko.txt")
				createNonKernelFile(tmpDir, "data.tar.gz")

				ok, err := containsKernelObjects.Validate(context.TODO(), image.ImageReference{ImageFSPath: tmpDir})
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})
		})

		Context("When .ko appears in filename but not as extension", func() {
			It("Should pass Validate", func() {
				tmpDir := setupKernelCheckTmpDir()
				createNonKernelFile(tmpDir, "config.ko.txt")
				createNonKernelFile(tmpDir, "readme.ko.bak")

				ok, err := containsKernelObjects.Validate(context.TODO(), image.ImageReference{ImageFSPath: tmpDir})
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})
		})

		AssertMetaData(&containsKernelObjects)
	})
})
