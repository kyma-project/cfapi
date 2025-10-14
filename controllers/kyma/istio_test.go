package kyma_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/cfapi/controllers/kyma"
	"github.com/kyma-project/cfapi/tools/k8s"
	"github.com/kyma-project/istio/operator/api/v1alpha2"
)

var _ = Describe("Istio", func() {
	var (
		istio                  *kyma.Istio
		alhpaGatewayAPIEnabled bool
		err                    error
	)

	BeforeEach(func() {
		istio = kyma.NewIstio(adminClient)
	})

	JustBeforeEach(func() {
		alhpaGatewayAPIEnabled, err = istio.IsAplhaGatewayAPIEnabled(ctx)
	})

	It("returns an error", func() {
		Expect(err).To(MatchError(ContainSubstring("failed to get the istio resource")))
	})

	When("the istio resource exists", func() {
		var istio *v1alpha2.Istio

		BeforeEach(func() {
			istio = &v1alpha2.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "kyma-system",
					Name:      "default",
				},
			}

			Expect(adminClient.Create(ctx, istio)).To(Succeed())
		})

		It("returns false", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(alhpaGatewayAPIEnabled).To(BeFalse())
		})

		When("any other istio experimental feature is enabled", func() {
			BeforeEach(func() {
				Expect(k8s.Patch(ctx, adminClient, istio, func() {
					istio.Spec.Experimental = &v1alpha2.Experimental{
						PilotFeatures: v1alpha2.PilotFeatures{
							EnableMultiNetworkDiscoverGatewayAPI: true,
						},
					}
				})).To(Succeed())
			})

			It("returns false", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(alhpaGatewayAPIEnabled).To(BeFalse())
			})
		})

		When("the alpha gateway api is enabled", func() {
			BeforeEach(func() {
				Expect(k8s.Patch(ctx, adminClient, istio, func() {
					istio.Spec.Experimental = &v1alpha2.Experimental{
						PilotFeatures: v1alpha2.PilotFeatures{
							EnableAlphaGatewayAPI: true,
						},
					}
				})).To(Succeed())
			})

			It("returns true", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(alhpaGatewayAPIEnabled).To(BeTrue())
			})
		})
	})
})
