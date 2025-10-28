package cfapi_test

import (
	"errors"

	"github.com/google/uuid"
	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/cfapi"
	"github.com/kyma-project/cfapi/controllers/installable"
	"github.com/kyma-project/cfapi/controllers/kyma"
	. "github.com/kyma-project/cfapi/tests/helpers"
	. "github.com/kyma-project/cfapi/tests/matchers"
	"github.com/kyma-project/cfapi/tools/k8s"
	"github.com/kyma-project/cfapi/tools/k8s/conditions"
	"github.com/kyma-project/istio/operator/api/v1alpha2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
		}
		Expect(adminClient.Create(ctx, cfAPI)).To(Succeed())
	})

	It("installs finalizer", func() {
		Eventually(func(g Gomega) {
			g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
			g.Expect(cfAPI.Finalizers).To(ContainElement(cfapi.Finalizer))
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

	It("installs installables", func() {
		Eventually(func(g Gomega) {
			g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())

			g.Expect(firstInstallable.InstallCallCount()).To(BeNumerically(">", 0))
			_, actualFirstInstallableConfig, _ := firstInstallable.InstallArgsForCall(firstInstallable.InstallCallCount() - 1)
			g.Expect(actualFirstInstallableConfig).To(Equal(cfAPI.Status.InstallationConfig))

			g.Expect(secondInstallable.InstallCallCount()).To(BeNumerically(">", 0))
			_, actualSecondInstallableConfig, _ := secondInstallable.InstallArgsForCall(secondInstallable.InstallCallCount() - 1)
			g.Expect(actualSecondInstallableConfig).To(Equal(cfAPI.Status.InstallationConfig))
		}).Should(Succeed())
	})

	It("sets install config on the status", func() {
		Eventually(func(g Gomega) {
			g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
			g.Expect(cfAPI.Status.InstallationConfig).To(Equal(v1alpha1.InstallationConfig{
				RootNamespace:           "cf",
				ContainerRegistrySecret: kyma.ContainerRegistrySecretName,
				CFDomain:                "kyma-host.com",
				UAAURL:                  "https://uaa.cf.eu12.hana.ondemand.com",
				CFAdmins:                []string{"default.admin@sap.com"},
			}))
		}).Should(Succeed())
	})

	It("sets the configuration status condition", func() {
		Eventually(func(g Gomega) {
			g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
			g.Expect(meta.IsStatusConditionTrue(cfAPI.Status.Conditions, v1alpha1.ConditionTypeConfiguration)).To(BeTrue())
		}).Should(Succeed())
	})

	It("sets the cf api url on the status", func() {
		Eventually(func(g Gomega) {
			g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
			g.Expect(cfAPI.Status.URL).To(Equal("https://cfapi.kyma-host.com"))
		}).Should(Succeed())
	})

	When("custom root namespace is specified", func() {
		BeforeEach(func() {
			Expect(k8s.Patch(ctx, adminClient, cfAPI, func() {
				cfAPI.Spec.RootNamespace = "custom-root-ns"
			})).To(Succeed())
		})

		It("uses it", func() {
			Eventually(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Status.InstallationConfig.RootNamespace).To(Equal("custom-root-ns"))
			}).Should(Succeed())
		})
	})

	When("custom uaa usr is specified", func() {
		BeforeEach(func() {
			Expect(k8s.Patch(ctx, adminClient, cfAPI, func() {
				cfAPI.Spec.UAA = "my-own.uaa.com"
			})).To(Succeed())
		})

		It("uses it", func() {
			Eventually(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Status.InstallationConfig.UAAURL).To(Equal("my-own.uaa.com"))
			}).Should(Succeed())
		})
	})

	When("custom admins are specified", func() {
		BeforeEach(func() {
			Expect(k8s.Patch(ctx, adminClient, cfAPI, func() {
				cfAPI.Spec.CFAdmins = []string{"custom-admin"}
			})).To(Succeed())
		})

		It("uses them", func() {
			Eventually(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Status.InstallationConfig.CFAdmins).To(ConsistOf("custom-admin"))
			}).Should(Succeed())
		})
	})

	When("korifi ingress service exists", func() {
		var ingressService *corev1.Service

		BeforeEach(func() {
			Expect(adminClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "korifi-gateway",
				},
			})).To(Succeed())

			ingressService = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "korifi-gateway",
					Name:      "korifi-istio",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{{
						Name:       "http",
						Protocol:   "TCP",
						Port:       8080,
						TargetPort: intstr.FromInt(8080),
					}},
				},
			}
			Expect(adminClient.Create(ctx, ingressService)).To(Succeed())
		})

		It("does not set the korifi ingress host", func() {
			EventuallyShouldHold(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Status.InstallationConfig.KorifiIngressHost).To(BeEmpty())
			})
		})

		When("the ingress service hostname is set", func() {
			BeforeEach(func() {
				Expect(k8s.Patch(ctx, adminClient, ingressService, func() {
					ingressService.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{
						Hostname: "korifi.host.com",
					}}
				})).To(Succeed())
			})

			It("sets the korifi ingress host", func() {
				Eventually(func(g Gomega) {
					g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
					g.Expect(cfAPI.Status.InstallationConfig.KorifiIngressHost).To(Equal("korifi.host.com"))
				}).Should(Succeed())
			})
		})

		When("the ingress service IP is set", func() {
			BeforeEach(func() {
				Expect(k8s.Patch(ctx, adminClient, ingressService, func() {
					ingressService.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{
						IP: "10.11.12.13",
					}}
				})).To(Succeed())
			})

			It("sets the korifi ingress host", func() {
				Eventually(func(g Gomega) {
					g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
					g.Expect(cfAPI.Status.InstallationConfig.KorifiIngressHost).To(Equal("10.11.12.13"))
				}).Should(Succeed())
			})
		})
	})

	It("sets ready state", func() {
		Eventually(func(g Gomega) {
			g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
			g.Expect(cfAPI.Status.State).To(Equal(v1alpha1.StateReady))
			g.Expect(cfAPI.Status.Conditions).To(ContainElement(SatisfyAll(
				HasType(Equal(v1alpha1.ConditionTypeInstallation)),
				HasStatus(Equal(metav1.ConditionTrue)),
				HasReason(Equal("InstallationSuccess")),
			)))
		}).Should(Succeed())
	})

	When("the cfapi spec is invalid", func() {
		BeforeEach(func() {
			Expect(k8s.Patch(ctx, adminClient, cfAPI, func() {
				cfAPI.Spec.ContainerRegistrySecret = uuid.NewString()
			})).To(Succeed())
		})

		It("sets warning state", func() {
			Eventually(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Status.State).To(Equal(v1alpha1.StateWarning))
				g.Expect(cfAPI.Status.Conditions).To(ContainElement(SatisfyAll(
					HasType(Equal(conditions.StatusConditionReady)),
					HasStatus(Equal(metav1.ConditionFalse)),
					HasReason(Equal("InvalidConfiguration")),
					HasMessage(ContainSubstring("not found")),
				)))
			}).Should(Succeed())
		})

		It("sets the configuration status condition to false", func() {
			Eventually(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(meta.IsStatusConditionFalse(cfAPI.Status.Conditions, v1alpha1.ConditionTypeConfiguration)).To(BeTrue())
			}).Should(Succeed())
		})
	})

	When("alpha gateway istio feature is not enabled", func() {
		BeforeEach(func() {
			istio := &v1alpha2.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "kyma-system",
					Name:      "default",
				},
			}
			Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(istio), istio)).To(Succeed())
			Expect(k8s.Patch(ctx, adminClient, istio, func() {
				istio.Spec.Experimental.EnableAlphaGatewayAPI = false
			})).To(Succeed())
		})

		It("sets warning state", func() {
			Eventually(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Status.State).To(Equal(v1alpha1.StateWarning))
				g.Expect(cfAPI.Status.Conditions).To(ContainElement(SatisfyAll(
					HasType(Equal(conditions.StatusConditionReady)),
					HasStatus(Equal(metav1.ConditionFalse)),
					HasReason(Equal("InvalidConfiguration")),
					HasMessage(ContainSubstring("not enabled")),
				)))
			}).Should(Succeed())
		})

		It("sets the configuration status condition to false", func() {
			Eventually(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(meta.IsStatusConditionFalse(cfAPI.Status.Conditions, v1alpha1.ConditionTypeConfiguration)).To(BeTrue())
			}).Should(Succeed())
		})
	})

	When("one of the installables returns an error", func() {
		BeforeEach(func() {
			secondInstallable.InstallReturns(installable.Result{}, errors.New("second-failed"))
		})

		It("leaves the CFAPI resource in processing state", func() {
			EventuallyShouldHold(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Status.State).To(Equal(v1alpha1.StateProcessing))
			})
		})
	})

	When("one of the installables returns a failure result", func() {
		BeforeEach(func() {
			secondInstallable.InstallReturns(installable.Result{
				State:   installable.ResultStateFailed,
				Message: "i have failed",
			}, nil)
		})

		It("sets error state in status", func() {
			Eventually(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Status.State).To(Equal(v1alpha1.StateError))
				g.Expect(cfAPI.Status.Conditions).To(ContainElement(SatisfyAll(
					HasType(Equal(v1alpha1.ConditionTypeInstallation)),
					HasStatus(Equal(metav1.ConditionFalse)),
					HasReason(Equal("InstallationFailed")),
					HasMessage(Equal("i have failed")),
				)))
			}).Should(Succeed())
		})
	})

	When("one of the installables returns processing result", func() {
		BeforeEach(func() {
			secondInstallable.InstallReturns(installable.Result{
				State: installable.ResultStateInProgress,
			}, nil)
		})

		It("sets processing state in status", func() {
			Eventually(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Status.State).To(Equal(v1alpha1.StateProcessing))
				g.Expect(cfAPI.Status.Conditions).To(ContainElement(SatisfyAll(
					HasType(Equal(v1alpha1.ConditionTypeInstallation)),
					HasStatus(Equal(metav1.ConditionUnknown)),
					HasReason(Equal("InstallationInProgress")),
				)))
			}).Should(Succeed())
		})
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
				cfAPI.Spec.ContainerRegistrySecret = customSecretName
			})).To(Succeed())
		})

		It("sets the custom container registry secret in the status", func() {
			Eventually(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Status.InstallationConfig.ContainerRegistrySecret).To(Equal(customSecretName))
			}).Should(Succeed())
		})

		When("the custom registry secret does not exists", func() {
			BeforeEach(func() {
				customSecretName = uuid.NewString()
				Expect(k8s.Patch(ctx, adminClient, cfAPI, func() {
					cfAPI.Spec.ContainerRegistrySecret = customSecretName
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

			It("sets the configuration status condition to false", func() {
				Eventually(func(g Gomega) {
					g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
					g.Expect(meta.IsStatusConditionFalse(cfAPI.Status.Conditions, v1alpha1.ConditionTypeConfiguration)).To(BeTrue())
				}).Should(Succeed())
			})
		})
	})
})
