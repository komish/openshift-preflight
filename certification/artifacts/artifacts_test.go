package artifacts_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-openshift-ecosystem/openshift-preflight/certification/artifacts"
)

var _ = Describe("Artifacts package context management", func() {
	Context("When working with an ArtifactWriter from context", func() {
		It("Should be settable and retrievable using helper functions", func() {
			ctx := context.Background()
			aw, err := artifacts.NewMapWriter()
			Expect(err).ToNot(HaveOccurred())

			ctx = artifacts.ContextWithWriter(ctx, aw)
			awRetrieved := artifacts.WriterFromContext(ctx)
			Expect(awRetrieved).ToNot(BeNil())
			Expect(awRetrieved).To(BeEquivalentTo(aw))
		})
	})
	It("Should return nil when there is no ArtifactWriter found in the context", func() {
		ctx := context.Background()
		awRetrieved := artifacts.WriterFromContext(ctx)
		Expect(awRetrieved).To(BeNil())
	})
})
