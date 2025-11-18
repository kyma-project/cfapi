package kyma_test

import (
	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/kyma"
	"github.com/kyma-project/cfapi/tools/k8s"
	istiov1alpha2 "github.com/kyma-project/istio/operator/api/v1alpha2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	networkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Gateway", func() {
	var (
		cfAPI   *v1alpha1.CFAPI
		gateway *kyma.Gateway
	)

	BeforeEach(func() {
		cfAPI = &v1alpha1.CFAPI{}
		gateway = kyma.NewGateway(adminClient)
	})

	Describe("KorifiGatewayType", func() {
		var gatewayType string

		JustBeforeEach(func() {
			gatewayType = gateway.KorifiGatewayType(cfAPI)
		})

		It("returns Contour as default gateway type", func() {
			Expect(gatewayType).To(Equal(v1alpha1.GatewayTypeContour))
		})

		When("the gateway type is set to istio", func() {
			BeforeEach(func() {
				cfAPI.Spec.GatewayType = v1alpha1.GatewayTypeIstio
			})

			It("returns Istio as gateway type", func() {
				Expect(gatewayType).To(Equal(v1alpha1.GatewayTypeIstio))
			})
		})

		When("the gateway type is set to contour", func() {
			BeforeEach(func() {
				cfAPI.Spec.GatewayType = v1alpha1.GatewayTypeContour
			})

			It("returns contour as gateway type", func() {
				Expect(gatewayType).To(Equal(v1alpha1.GatewayTypeContour))
			})
		})
	})

	Describe("Validate", func() {
		var validateErr error

		JustBeforeEach(func() {
			validateErr = gateway.Validate(ctx, cfAPI)
		})

		It("succeeds", func() {
			Expect(validateErr).NotTo(HaveOccurred())
		})

		When("the gateway type is set to Istio", func() {
			var istio *istiov1alpha2.Istio
			BeforeEach(func() {
				cfAPI.Spec.GatewayType = v1alpha1.GatewayTypeIstio

				istio = &istiov1alpha2.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kyma-system",
						Name:      "default",
					},
				}
				Expect(adminClient.Create(ctx, istio)).To(Succeed())
			})

			It("returns an error about alpha gateway API not being enabled", func() {
				Expect(validateErr).To(MatchError(ContainSubstring("alpha gateway API feature is not enabled in istio")))
			})

			When("the alpha gateway API feature is enabled in Istio", func() {
				BeforeEach(func() {
					Expect(k8s.Patch(ctx, adminClient, istio, func() {
						istio.Spec.Experimental = &istiov1alpha2.Experimental{
							PilotFeatures: istiov1alpha2.PilotFeatures{
								EnableAlphaGatewayAPI: true,
							},
						}
					})).To(Succeed())
				})

				It("succeeds", func() {
					Expect(validateErr).NotTo(HaveOccurred())
				})
			})
		})
	})

	Describe("KymaDomain", func() {
		var (
			kymaGateway *networkingv1beta1.Gateway
			domain      string
			domainErr   error
		)

		JustBeforeEach(func() {
			domain, domainErr = gateway.KymaDomain(ctx)
		})

		It("errors", func() {
			Expect(domainErr).To(MatchError(ContainSubstring("failed to get the kyma system gateway")))
		})

		When("the kyma gateway exists", func() {
			BeforeEach(func() {
				kymaGateway = &networkingv1beta1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kyma-system",
						Name:      "kyma-gateway",
					},
				}
				Expect(adminClient.Create(ctx, kymaGateway)).To(Succeed())
			})

			It("errors", func() {
				Expect(domainErr).To(MatchError(ContainSubstring("gateway has no servers")))
			})

			When("the kyma gateway has servers", func() {
				BeforeEach(func() {
					Expect(k8s.Patch(ctx, adminClient, kymaGateway, func() {
						kymaGateway.Spec.Servers = []*networkingv1alpha3.Server{
							{
								Port: &networkingv1alpha3.Port{
									Name:     "https-8080",
									Number:   8080,
									Protocol: "HTTP",
								},
								Hosts: []string{"*.kyma-domain.com"},
							},
						}
					})).To(Succeed())
				})

				It("returns the kyma domain", func() {
					Expect(domainErr).NotTo(HaveOccurred())
					Expect(domain).To(Equal("kyma-domain.com"))
				})
			})
		})
	})

	Describe("KorifiIngressService", func() {
		var ingressHostname string

		JustBeforeEach(func() {
			ingressHostname = gateway.KorifiIngressService(cfAPI)
		})

		It("returns the contour service", func() {
			Expect(ingressHostname).To(Equal("contour-envoy"))
		})

		When("the gateway type is set to Istio", func() {
			BeforeEach(func() {
				cfAPI.Spec.GatewayType = v1alpha1.GatewayTypeIstio
			})

			It("returns the istio service", func() {
				Expect(ingressHostname).To(Equal("korifi-istio"))
			})
		})
	})
})
