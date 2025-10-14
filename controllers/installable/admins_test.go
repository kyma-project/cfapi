package installable_test

import (
	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/installable"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Admins", func() {
	var (
		installConfig v1alpha1.InstallationConfig
		admins        *installable.Admins
		result        installable.Result
		err           error
	)

	BeforeEach(func() {
		installConfig = v1alpha1.InstallationConfig{
			RootNamespace: testNamespace,
			CFAdmins:      []string{"admin-user"},
		}

		admins = installable.NewAdmins(adminClient)
	})

	JustBeforeEach(func() {
		result, err = admins.Install(ctx, installConfig, eventRecorder)
	})

	It("creates the admin role bindings", func() {
		Expect(err).NotTo(HaveOccurred())
		Expect(result.State).To(Equal(installable.ResultStateSuccess))
		roleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cfapi-admins",
				Namespace: testNamespace,
			},
		}

		Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(roleBinding), roleBinding)).To(Succeed())
		Expect(roleBinding.Subjects).To(ConsistOf(rbacv1.Subject{
			Kind:      "User",
			APIGroup:  "rbac.authorization.k8s.io",
			Name:      "sap.ids:admin-user",
			Namespace: testNamespace,
		}))
		Expect(roleBinding.RoleRef).To(Equal(rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "korifi-controllers-admin",
		}))
		Expect(roleBinding.Annotations).To(HaveKeyWithValue("cloudfoundry.org/propagate-cf-role", "true"))
	})

	When("admin users already have oidc prefix", func() {
		BeforeEach(func() {
			installConfig.CFAdmins = []string{"sap.ids:admin-user"}
		})

		It("creates the admin role bindings with correct user", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.State).To(Equal(installable.ResultStateSuccess))
			roleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cfapi-admins",
					Namespace: testNamespace,
				},
			}

			Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(roleBinding), roleBinding)).To(Succeed())
			Expect(roleBinding.Subjects).To(ConsistOf(rbacv1.Subject{
				Kind:      "User",
				APIGroup:  "rbac.authorization.k8s.io",
				Name:      "sap.ids:admin-user",
				Namespace: testNamespace,
			}))
		})
	})

	When("the role binding already exists", func() {
		BeforeEach(func() {
			Expect(adminClient.Create(ctx, &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cfapi-admins",
					Namespace: testNamespace,
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "korifi-controllers-admin",
				},
			})).To(Succeed())
		})

		It("updates the binding", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.State).To(Equal(installable.ResultStateSuccess))
			roleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cfapi-admins",
					Namespace: testNamespace,
				},
			}

			Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(roleBinding), roleBinding)).To(Succeed())
			Expect(roleBinding.Subjects).To(ConsistOf(rbacv1.Subject{
				Kind:      "User",
				APIGroup:  "rbac.authorization.k8s.io",
				Name:      "sap.ids:admin-user",
				Namespace: testNamespace,
			}))
		})
	})
})
