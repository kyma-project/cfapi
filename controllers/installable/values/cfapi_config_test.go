package values_test

import (
	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/installable/values"
	"github.com/kyma-project/cfapi/tests/helpers"
	"github.com/kyma-project/cfapi/tools/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("CFAPI Config", func() {
	var (
		ingressService *corev1.Service
		cfAPIConfig    *values.CFAPIConfig
		instCfg        v1alpha1.InstallationConfig
		helmValues     map[string]any
		getValuesErr   error
	)

	BeforeEach(func() {
		instCfg = v1alpha1.InstallationConfig{
			KorifiIngressService: "contour-envoy",
			CFDomain:             "korifi.example.com",
			UAAURL:               "https://uaa.example.com",
			RootNamespace:        "my-root-ns",
			CFAdmins:             []string{"cf-admin@example.com"},
		}

		ingressService = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "cfapi-system",
				Name:      "contour-envoy",
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
				Ports: []corev1.ServicePort{{
					Port: 80,
				}},
			},
		}
		helpers.EnsureCreate(adminClient, ingressService)

		Expect(k8s.Patch(ctx, adminClient, ingressService, func() {
			ingressService.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
				{
					Hostname: "contour-enoy",
				},
			}
		})).To(Succeed())

		cfAPIConfig = values.NewCFAPIConfig(adminClient)
	})

	JustBeforeEach(func() {
		helmValues, getValuesErr = cfAPIConfig.GetValues(ctx, instCfg)
	})

	It("returns helm values", func() {
		Expect(getValuesErr).NotTo(HaveOccurred())
		Expect(helmValues).To(MatchAllKeys(Keys{
			"cfDomain":          Equal("korifi.example.com"),
			"korifiIngressHost": Equal("contour-enoy"),
			"uaaUrl":            Equal("https://uaa.example.com"),
			"rootNamespace":     Equal("my-root-ns"),
			"cfapiAdmins":       ConsistOf(Equal("sap.ids:cf-admin@example.com")),
		}))
	})

	When("the admin user is prefixed with sap.ids", func() {
		BeforeEach(func() {
			instCfg.CFAdmins = []string{"sap.ids:cf-admin@example.com"}
		})

		It("does not add the prefix again", func() {
			Expect(getValuesErr).NotTo(HaveOccurred())
			Expect(helmValues).To(MatchKeys(IgnoreExtras, Keys{
				"cfapiAdmins": ConsistOf(Equal("sap.ids:cf-admin@example.com")),
			}))
		})
	})

	When("the korifi ingress service has an IP instead of a hostname", func() {
		BeforeEach(func() {
			Expect(k8s.Patch(ctx, adminClient, ingressService, func() {
				ingressService.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
					{
						IP: "10.11.12.13",
					},
				}
			})).To(Succeed())
		})

		It("returns the IP as the korifi ingress host", func() {
			Expect(getValuesErr).NotTo(HaveOccurred())
			Expect(helmValues).To(MatchKeys(IgnoreExtras, Keys{
				"korifiIngressHost": Equal("10.11.12.13"),
			}))
		})
	})

	When("the korifi ingress service does not exist", func() {
		BeforeEach(func() {
			instCfg.KorifiIngressService = "non-existent-service"
		})

		It("returns an error", func() {
			Expect(getValuesErr).To(MatchError(ContainSubstring("failed to get korifi ingress service")))
		})
	})

	When("the korifi ingress service does not have an ingress assigned yet", func() {
		BeforeEach(func() {
			Expect(k8s.Patch(ctx, adminClient, ingressService, func() {
				ingressService.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{}
			})).To(Succeed())
		})

		It("returns an error", func() {
			Expect(getValuesErr).To(MatchError(ContainSubstring("korifi ingress service does not have an ingress assigned yet")))
		})
	})
})
