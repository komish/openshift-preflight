package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/artifacts"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/engine"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/formatters"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/policy"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/pyxis"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/runtime"

	"github.com/sirupsen/logrus"
)

var _ = Describe("Check Container Command", func() {
	var cmdPkgTestBaseDir string
	var artifactsDir string

	BeforeEach(func() {
		// instantiate err to make sure we can equal-assign in the following line.
		var err error
		cmdPkgTestBaseDir, err = os.MkdirTemp(os.TempDir(), "cmd-pkg-test-*")
		Expect(err).ToNot(HaveOccurred())
		Expect(len(cmdPkgTestBaseDir)).ToNot(BeZero())
		artifactsDir = path.Join(cmdPkgTestBaseDir, "artifacts")
		artifacts.SetDir(artifactsDir)
	})

	AfterEach(func() {
		err := os.RemoveAll(cmdPkgTestBaseDir)
		Expect(err).ToNot(HaveOccurred())
		artifacts.Reset()
	})

	Context("When determining container policy exceptions", func() {
		var fakePC *fakePyxisClient
		BeforeEach(func() {
			// reset the fake pyxis client before each execution
			// as a precaution.
			fakePC = &fakePyxisClient{
				findImagesByDigestFunc: fidbFuncNoop,
				getProjectsFunc:        gpFuncNoop,
				submitResultsFunc:      srFuncNoop,
			}
		})

		It("should throw an error if unable to get the project from the API", func() {
			fakePC.getProjectsFunc = gpFuncReturnError
			_, err := getContainerPolicyExceptions(context.TODO(), fakePC)
			Expect(err).To(HaveOccurred())
		})

		It("should return a scratch policy exception if the project has the flag in the API", func() {
			fakePC.getProjectsFunc = gpFuncReturnScratchException
			p, err := getContainerPolicyExceptions(context.TODO(), fakePC)
			Expect(p).To(Equal(policy.PolicyScratch))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return a root policy exception if the project has the flag in the API", func() {
			fakePC.getProjectsFunc = gpFuncReturnRootException
			p, err := getContainerPolicyExceptions(context.TODO(), fakePC)
			Expect(p).To(Equal(policy.PolicyRoot))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return a container policy exception if the project no exceptions in the API", func() {
			fakePC.getProjectsFunc = gpFuncReturnNoException
			p, err := getContainerPolicyExceptions(context.TODO(), fakePC)
			Expect(p).To(Equal(policy.PolicyContainer))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("When running the check container logic", func() {
		// use a local temporary directory so that we can make sure
		// our changes to the artifacts dir are specific to this test
		// and not being managed at the top level BeforeEach
		var localTempDir string
		var localArtifactsDir string
		var cfg *runtime.Config
		var pc pyxisClient
		var eng engine.CheckEngine
		var fmttr formatters.ResponseFormatter
		var rw resultWriter
		var rs resultSubmitter

		BeforeEach(func() {
			// instantiate err to make sure we can equal-assign in the following line.
			var err error
			localTempDir, err = os.MkdirTemp(os.TempDir(), "check-container-local-tempdir-*")
			Expect(err).ToNot(HaveOccurred())
			Expect(len(localTempDir)).ToNot(BeZero())
			localArtifactsDir = path.Join(localTempDir, "artifacts")
			// Don't set the artifacts dir here! This is handled by the function under test.

			img := "quay.io/example/foo:latest"
			// create a base config
			cfg = &runtime.Config{
				Image:     img,
				Artifacts: localArtifactsDir,
			}

			pc = &fakePyxisClient{
				findImagesByDigestFunc: fidbFuncNoop,
				getProjectsFunc:        gpFuncNoop,
				submitResultsFunc:      srFuncNoop,
			}

			eng = fakeCheckEngine{
				image:  img,
				passed: true,
			}

			fmttr, _ = formatters.NewByName(formatters.DefaultFormat)
			rw = &runtime.ResultWriterFile{}
			rs = &noopSubmitter{}
		})

		AfterEach(func() {
			err := os.RemoveAll(localTempDir)
			Expect(err).ToNot(HaveOccurred())
			artifacts.Reset()
		})

		Context("with a customized artifacts directory", func() {
			It("should set the artifacts directory accordingly", func() {
				// it's possible this will throw an error, but we dont' care for this test.
				_ = newCheckContainerFunc(context.TODO(), cfg, pc, eng, fmttr, rw, rs)
				Expect(artifacts.Path()).To(Equal(localArtifactsDir))
			})
		})

		Context("and the results file fails to open", func() {
			BeforeEach(func() {
				rw = &badResultWriter{errmsg: "some result writer error"}
			})

			It("should throw an error", func() {
				err := newCheckContainerFunc(context.TODO(), cfg, pc, eng, fmttr, rw, rs)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("some result writer error"))
			})
		})

		Context("with an engine that encounters an error while executing checks", func() {
			var msg string
			BeforeEach(func() {
				msg = "some internal engine error"
				eng = fakeCheckEngine{errorRunningChecks: true, errorMsg: msg}
			})
			It("should thrown an error", func() {
				err := newCheckContainerFunc(context.TODO(), cfg, pc, eng, fmttr, rw, rs)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(msg))
			})
		})

		Context("with a formatter that cannot properly format the results", func() {
			var msg string
			BeforeEach(func() {
				msg = "some error formatting results"
				fmttr = &badFormatter{errormsg: msg}
			})

			It("should throw an error", func() {
				err := newCheckContainerFunc(context.TODO(), cfg, pc, eng, fmttr, rw, rs)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(msg))
			})
		})

		Context("and the user has requested JUnit output", func() {
			BeforeEach(func() {
				cfg.WriteJUnit = true
			})
			It("should write a junit file in the artifacts directory", func() {
				err := newCheckContainerFunc(context.TODO(), cfg, pc, eng, fmttr, rw, rs)
				Expect(err).ToNot(HaveOccurred())
				Expect(path.Join(artifacts.Path(), "results-junit.xml")).To(BeAnExistingFile())
			})
		})

		Context("and submission encounteres an error", func() {
			var msg string
			BeforeEach(func() {
				msg = "some error submitting"
				rs = &badResultSubmitter{errmsg: msg}
				// TODO(): This is the package level variable, and isn't fantastic to have to evaluate in tests.
				// It would make sense to rely solely on the cfg.Submit value instead of the global variable.
				cfg.Submit = true
			})

			It("should throw an error", func() {
				err := newCheckContainerFunc(context.TODO(), cfg, pc, eng, fmttr, rw, rs)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(msg))
			})
		})

		Context("and there are no errors encountered in execution", func() {
			BeforeEach(func() {
				cfg.Submit = true
			})

			It("should complete with no errors", func() {
				err := newCheckContainerFunc(context.TODO(), cfg, pc, eng, fmttr, rw, rs)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("When using the containerCertificationSubmitter", func() {
		var sbmt *containerCertificationSubmitter
		var fakePC *fakePyxisClient
		var dockerConfigPath string
		var preflightLogPath string

		preflightLogFilename := "preflight.log"
		dockerconfigFilename := "dockerconfig.json"
		BeforeEach(func() {
			dockerConfigPath = path.Join(artifactsDir, dockerconfigFilename)
			preflightLogPath = path.Join(artifactsDir, preflightLogFilename)
			// Normalize a fakePyxisClient with noop functions.
			fakePC = &fakePyxisClient{
				findImagesByDigestFunc: fidbFuncNoop,
				getProjectsFunc:        gpFuncNoop,
				submitResultsFunc:      srFuncNoop,
			}

			// Most tests will need a passing getProjects func so set that to
			// avoid having to perform multiple BeforeEaches
			fakePC.setGPFuncReturnBaseProject("")

			// configure the submitter
			sbmt = &containerCertificationSubmitter{
				certificationProjectID: fakePC.baseProject("").ID,
				pyxis:                  fakePC,
				dockerConfig:           dockerConfigPath,
				preflightLogFile:       preflightLogPath,
			}

			certImageJSONBytes, err := json.Marshal(pyxis.CertImage{
				ID: "111111111111",
			})
			Expect(err).ToNot(HaveOccurred())

			preflightTestResultsJSONBytes, err := json.Marshal(runtime.Results{
				TestedImage:   "foo",
				PassedOverall: true,
			})
			Expect(err).ToNot(HaveOccurred())

			rpmManifestJSONBytes, err := json.Marshal(pyxis.RPMManifest{
				ID:      "foo",
				ImageID: "foo",
			})
			Expect(err).ToNot(HaveOccurred())

			// Create expected files.
			artifacts.WriteFile(dockerconfigFilename, "dockerconfig")
			artifacts.WriteFile(preflightLogFilename, "preflight log")
			artifacts.WriteFile(certification.DefaultCertImageFilename, string(certImageJSONBytes))
			artifacts.WriteFile(certification.DefaultTestResultsFilename, string(preflightTestResultsJSONBytes))
			artifacts.WriteFile(certification.DefaultRPMManifestFilename, string(rpmManifestJSONBytes))
		})

		Context("and project cannot be obtained from the API", func() {
			BeforeEach(func() {
				fakePC.getProjectsFunc = gpFuncReturnError
			})
			It("should throw an error", func() {
				err := sbmt.Submit(context.TODO())
				Expect(err).To(HaveOccurred())
			})
		})

		Context("and the provided docker config cannot be read from disk", func() {
			It("should throw an error", func() {
				err := os.Remove(dockerConfigPath)
				Expect(err).ToNot(HaveOccurred())

				err = sbmt.Submit(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(dockerconfigFilename))
			})
		})

		Context("and the cert image cannot be read from disk", func() {
			It("should throw an error", func() {
				err := os.Remove(path.Join(artifactsDir, certification.DefaultCertImageFilename))
				Expect(err).ToNot(HaveOccurred())

				err = sbmt.Submit(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(certification.DefaultCertImageFilename))
			})
		})

		Context("and the preflight results cannot be read from disk", func() {
			It("should throw an error", func() {
				err := os.Remove(path.Join(artifactsDir, certification.DefaultTestResultsFilename))
				Expect(err).ToNot(HaveOccurred())

				err = sbmt.Submit(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(certification.DefaultTestResultsFilename))
			})
		})

		Context("and the rpmManifest cannot be read from disk", func() {
			It("should throw an error", func() {
				err := os.Remove(path.Join(artifactsDir, certification.DefaultRPMManifestFilename))
				Expect(err).ToNot(HaveOccurred())

				err = sbmt.Submit(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(certification.DefaultRPMManifestFilename))
			})
		})

		Context("and the preflight logfile cannot be read from disk", func() {
			It("should throw an error", func() {
				err := os.Remove(preflightLogPath)
				Expect(err).ToNot(HaveOccurred())

				err = sbmt.Submit(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(preflightLogFilename))
			})
		})

		Context("and the submission fails", func() {
			BeforeEach(func() {
				fakePC.submitResultsFunc = srFuncReturnError
			})

			It("should throw an error", func() {
				err := sbmt.Submit(context.TODO())
				Expect(err).To(HaveOccurred())
			})
		})

		Context("and the certproject returned from pyxis is nil, but no error was returned", func() {
			BeforeEach(func() {
				fakePC.getProjectsFunc = gpFuncNoop
			})

			It("should throw an error", func() {
				err := sbmt.Submit(context.TODO())
				Expect(err).To(HaveOccurred())
			})
		})

		Context("and one of the submission artifacts is malformed", func() {
			BeforeEach(func() {
				artifacts.WriteFile(certification.DefaultRPMManifestFilename, "malformed")
			})

			It("should throw an error finalizing the submission", func() {
				err := sbmt.Submit(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to finalize data"))
			})
		})

		Context("and the submission succeeds", func() {
			BeforeEach(func() {
				fakePC.setSRFuncSubmitSuccessfully("", "")
			})
			It("should not throw an error", func() {
				err := sbmt.Submit(context.TODO())
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("When using the noop submitter", func() {
		var bf *bytes.Buffer
		var noop *noopSubmitter

		BeforeEach(func() {
			bufferLogger := logrus.New()
			bf = bytes.NewBuffer([]byte{})
			bufferLogger.SetOutput(bf)

			noop = &noopSubmitter{log: bufferLogger}
		})

		Context("and enabling log emitting", func() {
			BeforeEach(func() {
				noop.emitLog = true
			})

			It("should include the reason in the emitted log if specified", func() {
				testReason := "test reason"
				noop.reason = testReason
				err := noop.Submit(context.TODO())
				Expect(err).ToNot(HaveOccurred())
				Expect(bf.String()).To(ContainSubstring(testReason))
			})

			It("should emit logs when calling submit", func() {
				err := noop.Submit(context.TODO())
				Expect(err).ToNot(HaveOccurred())
				Expect(bf.String()).To(ContainSubstring("Results are not being sent for submission."))
			})
		})

		Context("and disabling log emitting", func() {
			It("should not emit logs when calling submit", func() {
				noop.emitLog = false
				err := noop.Submit(context.TODO())
				Expect(err).ToNot(HaveOccurred())
				Expect(bf.String()).To(BeEmpty())
			})
		})
	})

	Context("When resolving the submitter", func() {
		Context("with a valid pyxis client", func() {
			cfg := runtime.Config{
				CertificationProjectID: "projectid",
				PyxisHost:              "host",
				PyxisAPIToken:          "apitoken",
				DockerConfig:           "dockercfg",
				LogFile:                "logfile",
			}

			pc := newPyxisClient(context.TODO(), cfg.ReadOnly())
			Expect(pc).ToNot(BeNil())

			It("should return a containerCertificationSubmitter", func() {
				submitter := resolveSubmitter(pc, cfg.ReadOnly())
				typed, ok := submitter.(*containerCertificationSubmitter)
				Expect(typed).ToNot(BeNil())
				Expect(ok).To(BeTrue())
			})
		})

		Context("With no pyxis client", func() {
			cfg := runtime.Config{}
			It("should return a no-op submitter", func() {
				submitter := resolveSubmitter(nil, cfg.ReadOnly())
				typed, ok := submitter.(*noopSubmitter)
				Expect(typed).ToNot(BeNil())
				Expect(ok).To(BeTrue())
			})
		})
	})

	Context("When establishing a pyxis client.", func() {
		Context("with none of the required values", func() {
			cfgNoCertProjectID := runtime.Config{}

			It("Should return a nil pyxis client", func() {
				pc := newPyxisClient(context.TODO(), cfgNoCertProjectID.ReadOnly())
				Expect(pc).To(BeNil())
			})
		})

		Context("Missing any of the required values", func() {
			cfgMissingCertProjectID := runtime.Config{
				PyxisHost:     "foo",
				PyxisAPIToken: "bar",
			}

			cfgMissingPyxisHost := runtime.Config{
				CertificationProjectID: "foo",
				PyxisAPIToken:          "bar",
			}

			cfgMissingPyxisAPIToken := runtime.Config{
				CertificationProjectID: "foo",
				PyxisHost:              "bar",
			}

			It("Should return a nil pyxis client", func() {
				pc := newPyxisClient(context.TODO(), cfgMissingCertProjectID.ReadOnly())
				Expect(pc).To(BeNil())

				pc = newPyxisClient(context.TODO(), cfgMissingPyxisHost.ReadOnly())
				Expect(pc).To(BeNil())

				pc = newPyxisClient(context.TODO(), cfgMissingPyxisAPIToken.ReadOnly())
				Expect(pc).To(BeNil())
			})
		})

		Context("With all the required values", func() {
			cfgValid := runtime.Config{
				CertificationProjectID: "foo",
				PyxisHost:              "bar",
				PyxisAPIToken:          "baz",
			}

			It("should return a pyxis client", func() {
				pc := newPyxisClient(context.TODO(), cfgValid.ReadOnly())
				Expect(pc).ToNot(BeNil())
			})
		})
	})

	Context("When validating check container arguments and flags", func() {
		Context("and the user provided more than 1 positional arg", func() {
			It("should fail to run", func() {
				_, err := executeCommand(rootCmd, "check", "container", "foo", "bar")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("and the user provided less than 1 positional arg", func() {
			It("should fail to run", func() {
				_, err := executeCommand(rootCmd, "check", "container")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("and the user has enabled the submit flag", func() {
			It("should cause the certification-project-id and pyxis-api-token flag to be required", func() {
				out, err := executeCommand(rootCmd, "check", "container", "--submit", "foo")
				Expect(err).To(HaveOccurred())
				Expect(out).To(ContainSubstring("required flag(s) \"%s\", \"%s\" not set", "certification-project-id", "pyxis-api-token"))
			})
		})
	})
})
