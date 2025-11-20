package kyma_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/cfapi/controllers/kyma"
	"github.com/kyma-project/cfapi/tests/helpers"
	"github.com/kyma-project/cfapi/tools/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("UAA", func() {
	var (
		uaa    *kyma.UAA
		uaaURL string
		err    error
	)

	BeforeEach(func() {
		uaa = kyma.NewUAA(adminClient)
	})

	JustBeforeEach(func() {
		uaaURL, err = uaa.GetURL(ctx)
	})

	It("returns an error", func() {
		Expect(err).To(MatchError(ContainSubstring("not found")))
	})

	When("the btp service manager operator secret exists", func() {
		var btpOperatorSecret *corev1.Secret

		BeforeEach(func() {
			btpOperatorSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "kyma-system",
					Name:      "sap-btp-service-operator",
				},
				StringData: map[string]string{
					"tokenurl": "https://worker1-q3zjpctt.authentication.eu12.hana.ondemand.com",
				},
			}
			helpers.EnsureCreate(adminClient, btpOperatorSecret)
		})

		It("gets the uaa url", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(uaaURL).To(Equal("https://uaa.cf.eu12.hana.ondemand.com"))
		})

		When("the btp operator secret does not have a tokenurl key", func() {
			BeforeEach(func() {
				Expect(k8s.Patch(ctx, adminClient, btpOperatorSecret, func() {
					btpOperatorSecret.Data = map[string][]byte{}
				})).To(Succeed())
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("does not contain key")))
			})
		})
	})
})
