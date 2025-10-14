package registry_test

import (
	"path/filepath"

	"github.com/kyma-project/cfapi/controllers/registry"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var _ = Describe("Kyma", func() {
	var kymaRegistry *registry.Kyma

	BeforeEach(func() {
		kymaRegistry = registry.NewKyma(adminClient, scheme.Scheme)
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
					Paths: []string{filepath.Join("..", "..", "dependencies", "kyma-docker-registry")},
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns secret not found error", func() {
				Expect(k8serrors.IsNotFound(getSecretErr)).To(BeTrue())
			})

			When("the dockerregistry-config-external secret exists", func() {
				BeforeEach(func() {
					Expect(adminClient.Create(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testNamespace,
							Name:      registry.KymaRegistrySecret,
						},
					})).To(Succeed())
				})

				It("returns the secret", func() {
					Expect(getSecretErr).NotTo(HaveOccurred())
					Expect(secret.Namespace).To(Equal(testNamespace))
					Expect(secret.Name).To(Equal(registry.KymaRegistrySecret))
				})
			})
		})
	})
})
