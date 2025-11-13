package values_test

import (
	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/installable/values"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Korifi", func() {
	var (
		korifi     *values.Korifi
		instCfg    v1alpha1.InstallationConfig
		helmValues map[string]any
		err        error
	)

	BeforeEach(func() {
		instCfg = v1alpha1.InstallationConfig{
			RootNamespace:             "my-root-ns",
			ContainerRegistrySecret:   "my-registry-secret",
			ContainerRegistryURL:      "my-registry.com",
			ContainerRepositoryPrefix: "my-registry.com/",
			BuilderRepository:         "my-registry.com/cfapi/kpack-builder",
			UAAURL:                    "https://uaa.example.com",
			CFDomain:                  "korifi.example.com",
		}

		korifi = values.NewKorifi(adminClient, testNamepace)
		for _, certSecret := range []string{
			"korifi-api-ingress-cert",
			"korifi-workloads-ingress-cert",
			"korifi-api-internal-cert",
			"korifi-controllers-webhook-cert",
		} {
			Expect(adminClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamepace,
					Name:      certSecret,
				},
			})).To(Succeed())
		}
	})

	JustBeforeEach(func() {
		helmValues, err = korifi.GetValues(ctx, instCfg)
	})

	It("returns helm values", func() {
		Expect(err).NotTo(HaveOccurred())
		Expect(helmValues).To(MatchAllKeys(Keys{
			"systemNamespace":              Equal("cfapi-system"),
			"adminUserName":                Equal("cf-admin"),
			"generateInternalCertificates": BeFalse(),
			"containerRegistrySecrets":     ConsistOf("my-registry-secret"),
			"containerRepositoryPrefix":    Equal("my-registry.com/"),
			"defaultAppDomainName":         Equal("apps.korifi.example.com"),
			"api": MatchAllKeys(Keys{
				"apiServer": MatchAllKeys(Keys{
					"url": Equal("cfapi." + instCfg.CFDomain),
				}),
				"uaaURL": Equal(instCfg.UAAURL),
			}),
			"kpackImageBuilder": MatchAllKeys(Keys{
				"builderRepository": Equal("my-registry.com/cfapi/kpack-builder"),
			}),
			"networking": MatchAllKeys(Keys{
				"gatewayClass": Equal("istio"),
			}),
			"experimental": MatchAllKeys(Keys{
				"managedServices": MatchAllKeys(Keys{
					"enabled": BeTrue(),
				}),
				"uaa": MatchAllKeys(Keys{
					"enabled": BeTrue(),
					"url":     Equal("https://uaa.example.com"),
				}),
			}),
		}))
	})

	When("a required cert secret does not exist", func() {
		BeforeEach(func() {
			certSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamepace,
					Name:      "korifi-controllers-webhook-cert",
				},
			}
			Expect(adminClient.Delete(ctx, certSecret)).To(Succeed())
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})
})
