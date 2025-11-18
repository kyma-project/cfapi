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
	"github.com/kyma-project/istio/operator/api/v1alpha2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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

			g.Expect(firstToInstall.InstallCallCount()).To(BeNumerically(">", 0))
			_, actualFirstInstallableConfig, _ := firstToInstall.InstallArgsForCall(firstToInstall.InstallCallCount() - 1)
			g.Expect(actualFirstInstallableConfig).To(Equal(cfAPI.Status.InstallationConfig))

			g.Expect(secondToInstall.InstallCallCount()).To(BeNumerically(">", 0))
			_, actualSecondInstallableConfig, _ := secondToInstall.InstallArgsForCall(secondToInstall.InstallCallCount() - 1)
			g.Expect(actualSecondInstallableConfig).To(Equal(cfAPI.Status.InstallationConfig))
		}).Should(Succeed())
	})

	It("does not call the uninstallables", func() {
		Consistently(func(g Gomega) {
			g.Expect(firstToUninstall.UninstallCallCount()).To(BeZero())
			g.Expect(secondToUninstall.UninstallCallCount()).To(BeZero())
		}).Should(Succeed())
	})

	It("sets install config on the status", func() {
		Eventually(func(g Gomega) {
			g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
			g.Expect(cfAPI.Status.InstallationConfig).To(Equal(v1alpha1.InstallationConfig{
				RootNamespace:             "cf",
				ContainerRegistrySecret:   kyma.ContainerRegistrySecretName,
				ContainerRegistryURL:      "https://kyma-registry.com",
				ContainerRepositoryPrefix: "https://kyma-registry.com/",
				BuilderRepository:         "https://kyma-registry.com/cfapi/kpack-builder",
				CFDomain:                  "kyma-host.com",
				UAAURL:                    "https://uaa.cf.eu12.hana.ondemand.com",
				CFAdmins:                  []string{"default.admin@sap.com"},
				GatewayType:               "contour",
				KorifiIngressService:      "contour-envoy",
				DisableContainerRegistrySecretPropagation: false,
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

	When("container registry secret propagation is disabled", func() {
		BeforeEach(func() {
			Expect(k8s.Patch(ctx, adminClient, cfAPI, func() {
				cfAPI.Spec.DisableContainerRegistrySecretPropagation = true
			})).To(Succeed())
		})

		It("uses it", func() {
			Eventually(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Status.InstallationConfig.DisableContainerRegistrySecretPropagation).To(BeTrue())
			}).Should(Succeed())
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
			}).Should(Succeed())
		})

		It("sets the configuration status condition to false", func() {
			Eventually(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(meta.IsStatusConditionFalse(cfAPI.Status.Conditions, v1alpha1.ConditionTypeConfiguration)).To(BeTrue())
			}).Should(Succeed())
		})
	})

	When("the gateway type is invalid", func() {
		It("errors", func() {
			Expect(k8s.Patch(ctx, adminClient, cfAPI, func() {
				cfAPI.Spec.GatewayType = "my-gateway-type"
			})).To(MatchError(ContainSubstring("Unsupported value")))
		})
	})

	When("the gateway type is 'istio'", func() {
		BeforeEach(func() {
			Expect(k8s.Patch(ctx, adminClient, cfAPI, func() {
				cfAPI.Spec.GatewayType = "istio"
			})).To(Succeed())
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
				}).Should(Succeed())
			})

			It("sets the configuration status condition to false", func() {
				Eventually(func(g Gomega) {
					g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
					g.Expect(meta.IsStatusConditionFalse(cfAPI.Status.Conditions, v1alpha1.ConditionTypeConfiguration)).To(BeTrue())
				}).Should(Succeed())
			})
		})

		It("sets the korifi ingress service", func() {
			Eventually(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Status.InstallationConfig.KorifiIngressService).To(Equal("korifi-istio"))
			}).Should(Succeed())
		})
	})

	When("one of the installables returns an error", func() {
		BeforeEach(func() {
			secondToInstall.InstallReturns(installable.Result{}, errors.New("second-failed"))
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
			secondToInstall.InstallReturns(installable.Result{
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
			secondToInstall.InstallReturns(installable.Result{
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
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte(`{"auths":{"https://my-custom-registry.com": {}}}`),
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
				g.Expect(cfAPI.Status.InstallationConfig.ContainerRegistryURL).To(Equal("https://my-custom-registry.com"))
			}).Should(Succeed())
		})

		When("the custom registry secret does not specify registries", func() {
			BeforeEach(func() {
				customSecretName = uuid.NewString()
				Expect(adminClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: cfAPINamespace,
						Name:      customSecretName,
					},
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`),
					},
				})).To(Succeed())

				Expect(k8s.Patch(ctx, adminClient, cfAPI, func() {
					cfAPI.Spec.ContainerRegistrySecret = customSecretName
				})).To(Succeed())
			})

			It("sets warning status on the cfapi", func() {
				Eventually(func(g Gomega) {
					g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
					g.Expect(cfAPI.Status.State).To(Equal(v1alpha1.StateWarning))
				}).Should(Succeed())
			})

			It("sets the configuration status condition to false", func() {
				Eventually(func(g Gomega) {
					g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
					g.Expect(meta.IsStatusConditionFalse(cfAPI.Status.Conditions, v1alpha1.ConditionTypeConfiguration)).To(BeTrue())
				}).Should(Succeed())
			})
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

	When("deleting the CFAPI resource", func() {
		var uninstConfig v1alpha1.InstallationConfig

		BeforeEach(func() {
			Eventually(func(g Gomega) {
				g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Finalizers).To(ContainElement(cfapi.Finalizer))
			}).Should(Succeed())

			uninstConfig = cfAPI.Status.InstallationConfig
		})

		JustBeforeEach(func() {
			Expect(k8sManager.GetClient().Delete(ctx, cfAPI)).To(Succeed())
		})

		It("uninstalls uninstallables", func() {
			Eventually(func(g Gomega) {
				err := adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)
				g.Expect(k8serrors.IsNotFound(err)).To(BeTrue())

				g.Expect(firstToUninstall.UninstallCallCount()).To(BeNumerically(">", 0))
				_, actualFirstUninstallableConfig, _ := firstToUninstall.UninstallArgsForCall(firstToUninstall.UninstallCallCount() - 1)
				g.Expect(actualFirstUninstallableConfig).To(Equal(uninstConfig))

				g.Expect(secondToUninstall.UninstallCallCount()).To(BeNumerically(">", 0))
				_, actualSecondUninstallableConfig, _ := secondToUninstall.UninstallArgsForCall(secondToUninstall.UninstallCallCount() - 1)
				g.Expect(actualSecondUninstallableConfig).To(Equal(uninstConfig))
			}).Should(Succeed())
		})

		When("an uninstallable returns an error", func() {
			BeforeEach(func() {
				firstToUninstall.UninstallReturns(installable.Result{}, errors.New("uninstall-failed"))
			})

			It("does not let the resource go", func() {
				Consistently(func(g Gomega) {
					g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				}).Should(Succeed())
			})

			It("does not invoke the next uninstallable", func() {
				Consistently(func(g Gomega) {
					g.Expect(secondToUninstall.UninstallCallCount()).To(BeZero())
				}).Should(Succeed())
			})
		})

		When("an uninstallable returns in progress", func() { //nolint:dupl
			BeforeEach(func() {
				firstToUninstall.UninstallReturns(installable.Result{
					State:   installable.ResultStateInProgress,
					Message: "i-am-uninstalling",
				}, nil)
			})

			It("does not let the resource go", func() {
				EventuallyShouldHold(func(g Gomega) {
					g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
					g.Expect(cfAPI.Status.State).To(Equal(v1alpha1.StateDeleting))
					g.Expect(cfAPI.Status.Conditions).To(ContainElement(SatisfyAll(
						HasType(Equal(v1alpha1.ConditionTypeDeletion)),
						HasStatus(Equal(metav1.ConditionFalse)),
						HasReason(Equal("DeletionInProgress")),
						HasMessage(Equal("i-am-uninstalling")),
					)))
				})
			})

			It("does not invoke the next uninstallable", func() {
				Consistently(func(g Gomega) {
					g.Expect(secondToUninstall.UninstallCallCount()).To(BeZero())
				}).Should(Succeed())
			})
		})

		When("an uninstallable returns failed", func() { //nolint:dupl
			BeforeEach(func() {
				firstToUninstall.UninstallReturns(installable.Result{
					State:   installable.ResultStateFailed,
					Message: "i-failed",
				}, nil)
			})

			It("does not let the resource go", func() {
				EventuallyShouldHold(func(g Gomega) {
					g.Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
					g.Expect(cfAPI.Status.State).To(Equal(v1alpha1.StateDeleting))
					g.Expect(cfAPI.Status.Conditions).To(ContainElement(SatisfyAll(
						HasType(Equal(v1alpha1.ConditionTypeDeletion)),
						HasStatus(Equal(metav1.ConditionFalse)),
						HasReason(Equal("DeletionInProgress")),
						HasMessage(Equal("i-failed")),
					)))
				})
			})

			It("does not invoke the next uninstallable", func() {
				Consistently(func(g Gomega) {
					g.Expect(secondToUninstall.UninstallCallCount()).To(BeZero())
				}).Should(Succeed())
			})
		})
	})
})
