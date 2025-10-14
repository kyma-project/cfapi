package cfapi_test

import (
	"github.com/google/uuid"
	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/cfapi"
	"github.com/kyma-project/cfapi/controllers/registry"
	. "github.com/kyma-project/cfapi/tests/matchers"
	"github.com/kyma-project/cfapi/tools/k8s"
	"github.com/kyma-project/cfapi/tools/k8s/conditions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("CFDomainReconciler Integration Tests", func() {
	var cfAPI *v1alpha1.CFAPI

	BeforeEach(func() {
		cfAPI = &v1alpha1.CFAPI{
			ObjectMeta: metav1.ObjectMeta{
				Name:      uuid.NewString(),
				Namespace: cfAPINamespace,
			},
			Spec: v1alpha1.CFAPISpec{
				RootNamespace: uuid.NewString(),
			},
		}
		Expect(adminClient.Create(ctx, cfAPI)).To(Succeed())
	})

	It("installs finalizer", func() {
		Eventually(func(g Gomega) {
			g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
			g.Expect(cfAPI.Finalizers).To(ContainElement(cfapi.Finalizer))
		}).Should(Succeed())
	})

	It("sets default processing state", func() {
		Eventually(func(g Gomega) {
			g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
			g.Expect(cfAPI.Status.State).To(Equal(v1alpha1.StateProcessing))
		}).Should(Succeed())
	})

	When("the object is being deleted", func() {
		BeforeEach(func() {
			Expect(adminClient.Delete(ctx, cfAPI)).To(Succeed())
		})

		It("is deleted", func() {
			Eventually(func(g Gomega) {
				err := adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)
				g.Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())
		})
	})

	It("sets the observed generation", func() {
		Eventually(func(g Gomega) {
			g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
			g.Expect(cfAPI.Status.ObservedGeneration).To(Equal(cfAPI.Generation))
		}).Should(Succeed())
	})
	// ensureDockerRegistry - install the kyma docker registry CRD, whens for when the docker registry secret exists or does not exist
	// check korifi pull secret - if using kyma registry, cfapi.status.registrysecret is set to `docker-registry-external`, or to whatever secret the user specified

	It("sets the kyma container registry secret in the status", func() {
		Eventually(func(g Gomega) {
			g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
			g.Expect(cfAPI.Status.ContainerRegistrySecret).To(Equal(registry.KymaRegistrySecret))
		}).Should(Succeed())
	})

	When("the user has specified a custom registry secret", func() {
		var customSecretName string

		BeforeEach(func() {
			customSecretName = uuid.NewString()
			Expect(adminClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: cfAPINamespace,
					Name:      customSecretName,
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte("{}"),
				},
			})).To(Succeed())

			Expect(k8s.Patch(ctx, adminClient, cfAPI, func() {
				cfAPI.Spec.AppImagePullSecret = customSecretName
			})).To(Succeed())
		})

		It("sets the custom container registry secret in the status", func() {
			Eventually(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Status.ContainerRegistrySecret).To(Equal(customSecretName))
			}).Should(Succeed())
		})

		When("the custom registry secret does not exists", func() {
			BeforeEach(func() {
				customSecretName = uuid.NewString()
				Expect(k8s.Patch(ctx, adminClient, cfAPI, func() {
					cfAPI.Spec.AppImagePullSecret = customSecretName
				})).To(Succeed())
			})

			It("sets warning status on the cfapi", func() {
				Eventually(func(g Gomega) {
					g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
					g.Expect(cfAPI.Status.State).To(Equal(v1alpha1.StateWarning))
					g.Expect(cfAPI.Status.Conditions).To(ContainElement(SatisfyAll(
						HasType(Equal(conditions.StatusConditionReady)),
						HasStatus(Equal(metav1.ConditionFalse)),
					)))
				}).Should(Succeed())
			})
		})
	})
})
