package installable_test

import (
	korifiv1alpha1 "code.cloudfoundry.org/korifi/controllers/api/v1alpha1"
	"github.com/google/uuid"
	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/installable"
	"github.com/kyma-project/cfapi/tests/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Uninstall", func() {
	var (
		orgInstallable *installable.Orgs
		config         v1alpha1.InstallationConfig

		result     installable.Result
		installErr error
	)

	BeforeEach(func() {
		config = v1alpha1.InstallationConfig{
			RootNamespace: testNamespace,
		}
		orgInstallable = installable.NewOrgs(adminClient)
	})

	JustBeforeEach(func() {
		result, installErr = orgInstallable.Uninstall(ctx, config, eventRecorder)
	})

	It("succeeds", func() {
		Expect(installErr).NotTo(HaveOccurred())
		Expect(result).To(Equal(installable.Result{
			State:   installable.ResultStateSuccess,
			Message: "Orgs deleted successfully",
		}))
	})

	When("orgs exist", func() {
		BeforeEach(func() {
			for range 3 {
				helpers.EnsureCreate(adminClient, &korifiv1alpha1.CFOrg{
					ObjectMeta: metav1.ObjectMeta{
						Name:      uuid.NewString(),
						Namespace: testNamespace,
					},
					Spec: korifiv1alpha1.CFOrgSpec{
						DisplayName: uuid.NewString(),
					},
				})
			}
		})

		JustBeforeEach(func() {
			Eventually(func(g Gomega) {
				result, installErr = orgInstallable.Uninstall(ctx, config, eventRecorder)
				g.Expect(installErr).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(installable.Result{
					State:   installable.ResultStateSuccess,
					Message: "Orgs deleted successfully",
				}))
			}).Should(Succeed())
		})

		It("deletes all orgs", func() {
			orgList := &korifiv1alpha1.CFOrgList{}
			Expect(adminClient.List(ctx, orgList, client.InNamespace(testNamespace))).To(Succeed())
			Expect(orgList.Items).To(BeEmpty())
		})
	})
})
