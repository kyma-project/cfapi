package kyma_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/cfapi/controllers/kyma"
	"github.com/kyma-project/cfapi/tools"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	istiov1alpha3 "istio.io/api/networking/v1alpha3"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	gwclient "istio.io/client-go/pkg/clientset/versioned/typed/networking/v1beta1"
)

var _ = Describe("Domain", func() {
	var (
		domain            *kyma.Domain
		kymaDefaultDomain string
		err               error
	)

	BeforeEach(func() {
		domain = kyma.NewDomain(istioclient.NewForConfigOrDie(testEnv.Config))
	})

	JustBeforeEach(func() {
		kymaDefaultDomain, err = domain.Get(ctx)
	})

	It("returns an error", func() {
		Expect(err).To(MatchError(ContainSubstring("not found")))
	})

	When("the kyma istio gateway exists", func() {
		var (
			kymaGateway   *istiov1beta1.Gateway
			gatewayClient gwclient.GatewayInterface
		)

		BeforeEach(func() {
			var istioClient *istioclient.Clientset
			istioClient, err = istioclient.NewForConfig(testEnv.Config)
			Expect(err).NotTo(HaveOccurred())

			gatewayClient = istioClient.NetworkingV1beta1().Gateways("kyma-system")

			kymaGateway = &istiov1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "kyma-system",
					Name:      "kyma-gateway",
				},
				Spec: istiov1alpha3.Gateway{
					Servers: []*istiov1alpha3.Server{{
						Hosts: []string{"*.my-gw-host.com"},
						Port: tools.PtrTo(istiov1alpha3.Port{
							Name:     "whatever",
							Number:   30111,
							Protocol: "https",
						}),
					}},
				},
			}

			kymaGateway, err = gatewayClient.Create(ctx, kymaGateway, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("gets the wildcard domain", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(kymaDefaultDomain).To(Equal("my-gw-host.com"))
		})

		When("the istio gateway has no servers", func() {
			BeforeEach(func() {
				kymaGateway.Spec.Servers = nil
				kymaGateway, err = gatewayClient.Update(ctx, kymaGateway, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("gateway has no servers")))
			})
		})
	})
})
