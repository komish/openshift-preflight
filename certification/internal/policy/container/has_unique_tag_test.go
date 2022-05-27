package container

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification"
)

var _ = Describe("UniqueTag", func() {
	var hasUniqueTagCheck hasUniqueTagCheck = *NewHasUniqueTagCheck("")
	var src, dst, host string

	BeforeEach(func() {
		// Set up a fake registry.
		registryLogger := log.New(io.Discard, "", log.Ldate)
		s := httptest.NewServer(registry.New(registry.Logger(registryLogger)))
		DeferCleanup(func() {
			s.Close()
		})
		u, err := url.Parse(s.URL)
		Expect(err).ToNot(HaveOccurred())
		src = fmt.Sprintf("%s/test/preflight", u.Host)
		dst = fmt.Sprintf("%s/test/tags", u.Host)
		host = u.Host

		img, err := random.Image(1024, 5)
		Expect(err).ToNot(HaveOccurred())

		err = crane.Push(img, src)
		Expect(err).ToNot(HaveOccurred())

		err = crane.Copy(src, dst)
		Expect(err).ToNot(HaveOccurred())

		err = crane.Tag(dst, "unique-tag")
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Checking for unique tags", func() {
		Context("When it has tags other than latest", func() {
			It("should pass Validate", func() {
				ok, err := hasUniqueTagCheck.Validate(context.TODO(), certification.ImageReference{ImageRegistry: host, ImageRepository: "test/tags"})
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})
		})
		Context("When it has only latest tag", func() {
			It("should not pass Validate", func() {
				ok, err := hasUniqueTagCheck.Validate(context.TODO(), certification.ImageReference{ImageRegistry: host, ImageRepository: "test/preflight"})
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())
			})
		})
		Context("When registry returns an empty tag list", func() {
			BeforeEach(func() {
				s := httptest.NewServer(http.HandlerFunc(mockRegistry))
				DeferCleanup(func() {
					s.Close()
				})
				u, err := url.Parse(s.URL)
				Expect(err).ToNot(HaveOccurred())
				host = u.Host
			})
			It("should pass Validate", func() {
				ok, err := hasUniqueTagCheck.Validate(context.TODO(), certification.ImageReference{ImageRegistry: host, ImageRepository: "test/notags", ImageTagOrSha: "v0.0.1"})
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})
		})
	})

	AssertMetaData(&hasUniqueTagCheck)
})

func validImageTags() []string {
	return []string{"0.0.1", "0.0.2", "latest"}
}

func invalidImageTags() []string {
	return []string{"latest"}
}

func emptyImageTags() []string {
	return []string{}
}

type tagsList struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// mockRegistry() is customized due to the way some partners don't expose
// the `/tags/list` API endpoint in their private registry implementations.
func mockRegistry(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	repo := "test/notags"
	tagURLPath := "/v2/" + repo + "/tags/list"
	if req.URL.Path != tagURLPath && req.URL.Path != tagURLPath+"/" && req.Method != "GET" {
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	resp.Header().Set("Content-Type", "application/json")

	tagsResp := tagsList{
		Name: repo,
		Tags: emptyImageTags(),
	}

	jbod, _ := json.Marshal(tagsResp)
	resp.Header().Set("Content-Length", fmt.Sprint(len(jbod)))
	resp.WriteHeader(http.StatusOK)
	io.Copy(resp, bytes.NewReader([]byte(jbod)))
	return
}
