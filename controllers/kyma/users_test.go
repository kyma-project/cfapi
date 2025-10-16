package kyma_test

import (
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/cfapi/controllers/kyma"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Users", func() {
	var (
		users  *kyma.Users
		admins []rbacv1.Subject
	)

	BeforeEach(func() {
		users = kyma.NewUsers(adminClient)
	})

	JustBeforeEach(func() {
		var err error
		admins, err = users.GetClusterAdmins(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an empty list", func() {
		Expect(admins).To(BeEmpty())
	})

	When("there are admin users", func() {
		BeforeEach(func() {
			Expect(adminClient.Create(ctx, &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: uuid.NewString(),
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      "admin-sa",
						Namespace: testNamespace,
					},
					{
						Kind:      "Group",
						Name:      "admin-group",
						Namespace: testNamespace,
					},
					{
						Kind:      "User",
						Name:      "admin-user",
						Namespace: testNamespace,
					},
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "cluster-admin",
				},
			})).To(Succeed())
		})

		It("returns user subjects only", func() {
			Expect(admins).To(ConsistOf(rbacv1.Subject{
				APIGroup:  "rbac.authorization.k8s.io",
				Kind:      "User",
				Name:      "admin-user",
				Namespace: testNamespace,
			}))
		})
	})
})
