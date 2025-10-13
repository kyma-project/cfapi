package create_test

import (
	"crypto/tls"
	"encoding/json"
	"net/http"

	gardenerv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers"
	. "github.com/kyma-project/cfapi/tests/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	buildv1alpha2 "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"gopkg.in/yaml.v3"

	v1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"

	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Integration", func() {
	Describe("CFAPI resource", func() {
		var cfAPI *v1alpha1.CFAPI

		BeforeEach(func() {
			cfAPI = &v1alpha1.CFAPI{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "cfapi-system",
					Name:      cfAPIName,
				},
			}
		})

		It("creates CFAPI resource with ready status", func() {
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())

				condition := meta.FindStatusCondition(cfAPI.Status.Conditions, v1alpha1.ConditionTypeInstallation)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Reason).To(Equal(v1alpha1.ConditionReasonReady))
				g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
		})

		It("sets the finalizer on the resource", func() {
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cfAPI), cfAPI)).To(Succeed())
				g.Expect(cfAPI.Finalizers).To(ConsistOf("sample.kyma-project.io/finalizer"))
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

	It("installs Gateway API custom resources", func() {
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.List(ctx, &gatewayv1.GatewayList{})).To(Succeed())
		}).Should(Succeed())
	})

	It("enables istio alpha gateway api", func() {
		Eventually(func(g Gomega) {
			istiodDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istiod",
					Namespace: "istio-system",
				},
			}

			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(istiodDeployment), istiodDeployment)).To(Succeed())
			g.Expect(istiodDeployment.Spec.Template.Spec.Containers[0].Env).To(ContainElements([]corev1.EnvVar{
				{Name: "PILOT_ENABLE_ALPHA_GATEWAY_API", Value: "true"},
			}))
		}).Should(Succeed())
	})

	It("installs the envoy filter", func() {
		Eventually(func(g Gomega) {
			envoyFilter := &v1alpha3.EnvoyFilter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ef-removeserver",
					Namespace: "istio-system",
				},
			}

			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(envoyFilter), envoyFilter)).To(Succeed())
		}).Should(Succeed())
	})

	DescribeTable("Registry secrets", func(ns string) {
		Eventually(func(g Gomega) {
			kymaRegistrySecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "cfapi-system",
					Name:      "dockerregistry-config-external",
				},
			}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(kymaRegistrySecret), kymaRegistrySecret)).To(Succeed())
			g.Expect(kymaRegistrySecret.Data).To(SatisfyAll(
				HaveKey(".dockerconfigjson"),
				HaveKey("pushRegAddr"),
				HaveKey("username"),
				HaveKey("password"),
			))

			korifiRegistrySecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cfapi-app-registry",
					Namespace: ns,
				},
			}

			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(korifiRegistrySecret), korifiRegistrySecret)).To(Succeed())
			g.Expect(korifiRegistrySecret.Type).To(Equal(corev1.SecretTypeDockerConfigJson))

			var korifiRegistryConfig controllers.DockerRegistryConfig
			g.Expect(json.Unmarshal(korifiRegistrySecret.Data[".dockerconfigjson"], &korifiRegistryConfig)).To(Succeed())
			g.Expect(korifiRegistryConfig.Auths).To(SatisfyAll(
				HaveLen(1),
				HaveKeyWithValue(string(kymaRegistrySecret.Data["pushRegAddr"]), controllers.DockerRegistryAuth{
					Username: string(kymaRegistrySecret.Data["username"]),
					Password: string(kymaRegistrySecret.Data["password"]),
				}),
			))
		}).Should(Succeed())
	},
		Entry("namespace korifi", "korifi"),
		Entry("namespace cf", "cf"),
	)

	DescribeTable("Certificates",
		func(certificateName string) {
			certificate := &gardenerv1alpha1.Certificate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      certificateName,
					Namespace: "korifi",
				},
			}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(certificate), certificate)).To(Succeed())
				g.Expect(certificate.Status.Conditions).To(ContainElement(SatisfyAll(
					HasType(Equal(string(gardenerv1alpha1.CertificateConditionReady))),
					HasStatus(Equal(metav1.ConditionTrue)),
					HasReason(Equal(string(v1alpha1.ConditionReasonReady))),
				)))

				g.Expect(k8sClient.Get(ctx, client.ObjectKey{
					Namespace: "korifi",
					Name:      certificateName,
				}, &corev1.Secret{})).To(Succeed())
			}).Should(Succeed())
		},
		Entry("korifi api ingress certificate", "korifi-api-ingress-cert"),
		Entry("korifi workloads ingress certificate", "korifi-workloads-ingress-cert"),
		Entry("korifi api internal certificate", "korifi-api-internal-cert"),
		Entry("korifi kpack webhook certificate", "korifi-kpack-image-builder-webhook-cert"),
		Entry("korifi controllers webhook certificate", "korifi-controllers-webhook-cert"),
		Entry("korifi statefulset webhook certificate", "korifi-statefulset-runner-webhook-cert"),
	)

	DescribeTable("Installed deployments", func(ns string, name string) {
		Eventually(func(g Gomega) {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
				},
			}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
			g.Expect(deployment.Status.ReadyReplicas).To(BeNumerically(">", 0))
		}).Should(Succeed())
	},
		Entry("kpack", "kpack", "kpack-controller"),
		Entry("btp service broker", "cfapi-system", "btp-service-broker"),
		Entry("korifi api", "korifi", "korifi-api-deployment"),
		Entry("korifi controllers manager", "korifi", "korifi-controllers-controller-manager"),
		Entry("korifi istio gateway", "korifi-gateway", "korifi-istio"),
		Entry("korifi job task runner", "korifi", "korifi-job-task-runner-controller-manager"),
		Entry("korifi kpack", "korifi", "korifi-kpack-image-builder-controller-manager"),
		Entry("korifi statefulset runner", "korifi", "korifi-statefulset-runner-controller-manager"),
	)

	Describe("Dns Entries", func() {
		It("creates api dns entry", func() {
			Eventually(func(g Gomega) {
				apiDNSEntry := &dnsv1alpha1.DNSEntry{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cf-api-ingress",
						Namespace: "korifi",
					},
				}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(apiDNSEntry), apiDNSEntry)).To(Succeed())
				g.Expect(apiDNSEntry.Spec.DNSName).To(Equal("cfapi.kind-127-0-0-1.nip.io"))
			}).Should(Succeed())
		})

		It("creates apps dns entry", func() {
			Eventually(func(g Gomega) {
				appsDNSEntry := &dnsv1alpha1.DNSEntry{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cf-apps-ingress",
						Namespace: "korifi",
					},
				}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(appsDNSEntry), appsDNSEntry)).To(Succeed())
				g.Expect(appsDNSEntry.Spec.DNSName).To(Equal("*.apps.kind-127-0-0-1.nip.io"))
			}).Should(Succeed())
		})
	})

	Describe("Korifi config", func() {
		It("creates correct api config", func() {
			Eventually(func(g Gomega) {
				korifiAPIConfigMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "korifi-api-config",
						Namespace: "korifi",
					},
				}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(korifiAPIConfigMap), korifiAPIConfigMap)).To(Succeed())
				rawData, ok := korifiAPIConfigMap.Data["korifi_api_config.yaml"]
				g.Expect(ok).To(BeTrue())

				var configMapData map[string]any
				g.Expect(yaml.Unmarshal([]byte(rawData), &configMapData)).To(Succeed())
				g.Expect(configMapData).To(HaveKeyWithValue("externalFQDN", "cfapi.kind-127-0-0-1.nip.io"))
				g.Expect(configMapData).To(HaveKeyWithValue("containerRepositoryPrefix", "registry-default-kyma-system.kind-127-0-0-1.nip.io/"))
				g.Expect(configMapData).To(HaveKeyWithValue("defaultDomainName", "apps.kind-127-0-0-1.nip.io"))
				g.Expect(configMapData).To(
					HaveKeyWithValue("experimental",
						HaveKeyWithValue("uaa", SatisfyAll(
							HaveKeyWithValue("enabled", true),
							HaveKeyWithValue("url", "https://uaa.cf.dummy-token.url")))))
			}).Should(Succeed())
		})

		It("configures the cluster builder", func() {
			Eventually(func(g Gomega) {
				clusterBuilder := buildv1alpha2.ClusterBuilder{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cf-kpack-cluster-builder",
					},
				}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&clusterBuilder), &clusterBuilder)).To(Succeed())
				g.Expect(clusterBuilder.Spec.Tag).To(Equal("registry-default-kyma-system.kind-127-0-0-1.nip.io/cfapi/kpack-builder"))
			}).Should(Succeed())
		})
	})

	It("assigns cluster admins the CFAdmin role", func() {
		Eventually(func(g Gomega) {
			cfAdminBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "cf",
					Name:      "cfapi-admins-binding",
				},
			}

			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cfAdminBinding), cfAdminBinding)).To(Succeed())
			g.Expect(cfAdminBinding.RoleRef.Name).To(Equal("korifi-controllers-admin"))
			g.Expect(cfAdminBinding.Subjects).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Name": Equal("sap.ids:" + sharedData.CfAdminUser),
			})))
		}).Should(Succeed())
	})
})
