package create_test

import (
	"crypto/tls"
	"net/http"

	"github.com/kyma-project/cfapi/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("CFAPI Create", func() {
	var cfAPI *v1alpha1.CFAPI

	BeforeEach(func() {
		cfAPI = &v1alpha1.CFAPI{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "cfapi-system",
				Name:      cfAPIName,
			},
		}
	})

	It("sets status state to Ready", func() {
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
			g.Expect(cfAPI.Status.State).To(Equal(v1alpha1.StateReady))
		}).Should(Succeed())
	})

	It("sets usable cfapi url", func() {
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
			g.Expect(cfAPI.Status.URL).NotTo(BeEmpty())

			http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
			resp, err := http.Get(cfAPI.Status.URL)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp.StatusCode).To(Equal(http.StatusOK))
		}).Should(Succeed())
	})
})
