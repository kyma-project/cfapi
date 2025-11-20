package kyma_test

import (
	"path/filepath"

	"github.com/kyma-project/cfapi/controllers/kyma"
	"github.com/kyma-project/cfapi/tests/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var _ = Describe("Kyma", func() {
	var kymaRegistry *kyma.ContainerRegistry

	BeforeEach(func() {
		kymaRegistry = kyma.NewContainerRegistry(adminClient)
	})

	Describe("GetRegistrySecret", func() {
		var (
			secret       *corev1.Secret
			getSecretErr error
		)

		JustBeforeEach(func() {
			secret, getSecretErr = kymaRegistry.GetRegistrySecret(ctx, testNamespace)
		})

		It("returns an error", func() {
			Expect(getSecretErr).To(MatchError(ContainSubstring("dockerregistry kyma module is not enabled")))
		})

		When("the docker registry custom resource exists", func() {
			BeforeEach(func() {
				_, err := envtest.InstallCRDs(testEnv.Config, envtest.CRDInstallOptions{
					Paths: []string{filepath.Join("..", "..", "tests", "dependencies", "vendor", "kyma-docker-registry")},
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns secret not found error", func() {
				Expect(k8serrors.IsNotFound(getSecretErr)).To(BeTrue())
			})

			When("the dockerregistry-config secret exists", func() {
				BeforeEach(func() {
					helpers.EnsureCreate(adminClient, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testNamespace,
							Name:      kyma.ContainerRegistrySecretName,
						},
					})
				})

				It("returns the secret", func() {
					Expect(getSecretErr).NotTo(HaveOccurred())
					Expect(secret.Namespace).To(Equal(testNamespace))
					Expect(secret.Name).To(Equal(kyma.ContainerRegistrySecretName))
				})
			})
		})
	})

	Describe("GetRegistryURL", func() {
		var (
			registryURL string
			err         error
			secretData  map[string]string
		)

		BeforeEach(func() {
			_, err = envtest.InstallCRDs(testEnv.Config, envtest.CRDInstallOptions{
				Paths: []string{filepath.Join("..", "..", "tests", "dependencies", "vendor", "kyma-docker-registry")},
			})
			Expect(err).NotTo(HaveOccurred())
			secretData = map[string]string{
				"pushRegAddr": "dockerregistry.kyma-system.svc.cluster.local:5000",
			}
		})

		JustBeforeEach(func() {
			helpers.EnsureCreate(adminClient, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace,
					Name:      kyma.ContainerRegistrySecretName,
				},
				StringData: secretData,
			})

			registryURL, err = kymaRegistry.GetRegistryURL(ctx, testNamespace)
		})

		It("returns the push registry url", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(registryURL).To(Equal("dockerregistry.kyma-system.svc.cluster.local:5000"))
		})

		When("the pushRegAddr key is missing", func() {
			BeforeEach(func() {
				secretData = map[string]string{}
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("pushRegAddr")))
			})
		})
	})
})
