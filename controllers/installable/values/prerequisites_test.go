package values_test

import (
	certv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/installable/values"
	"github.com/kyma-project/cfapi/controllers/kyma"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Prerequisites", func() {
	var (
		prerequisites *values.Prerequisites
		instCfg       v1alpha1.InstallationConfig
		helmValues    map[string]any
		err           error
	)

	BeforeEach(func() {
		Expect(adminClient.Create(ctx, &certv1alpha1.Issuer{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "kyma-system",
				Name:      "cfapi-self-signed-issuer",
			},
			Spec: certv1alpha1.IssuerSpec{
				SelfSigned: &certv1alpha1.SelfSignedSpec{},
			},
		})).To(Succeed())

		instCfg = v1alpha1.InstallationConfig{
			CFDomain:                  "korifi.example.com",
			UseSelfSignedCertificates: true,
			ContainerRegistrySecret:   kyma.ContainerRegistrySecretName,
			RootNamespace:             "my-root-ns",
			GatewayType:               "contour",
		}

		prerequisites = values.NewPrerequisites(adminClient)
	})

	JustBeforeEach(func() {
		helmValues, err = prerequisites.GetValues(ctx, instCfg)
	})

	It("returns helm values", func() {
		Expect(err).NotTo(HaveOccurred())
		Expect(helmValues).To(MatchAllKeys(Keys{
			"systemNamespace":           Equal("kyma-system"),
			"cfDomain":                  Equal("korifi.example.com"),
			"useSelfSignedCertificates": Equal(true),
			"selfSignedIssuer":          Equal("cfapi-self-signed-issuer"),
			"gatewayType":               Equal("contour"),
			"containerRegistrySecret": MatchAllKeys(Keys{
				"name": Equal(kyma.ContainerRegistrySecretName),
				"propagation": MatchAllKeys(Keys{
					"enabled": Equal(false),
				}),
			}),
		}))
	})

	When("the container registry secret is not the kyma registry one", func() {
		BeforeEach(func() {
			instCfg.ContainerRegistrySecret = "custom-registry-secret"
		})

		It("returns helm values", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(helmValues).To(MatchKeys(IgnoreExtras, Keys{
				"containerRegistrySecret": MatchAllKeys(Keys{
					"name": Equal("custom-registry-secret"),
					"propagation": MatchAllKeys(Keys{
						"enabled":              Equal(true),
						"sourceNamespace":      Equal("cfapi-system"),
						"destinationNamespace": Equal("my-root-ns"),
					}),
				}),
			}))
		})

		When("container registry secret propagation is disabled", func() {
			BeforeEach(func() {
				instCfg.DisableContainerRegistrySecretPropagation = true
			})

			It("returns helm values", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(helmValues).To(MatchKeys(IgnoreExtras, Keys{
					"containerRegistrySecret": MatchAllKeys(Keys{
						"name": Equal("custom-registry-secret"),
						"propagation": MatchAllKeys(Keys{
							"enabled": Equal(false),
						}),
					}),
				}))
			})
		})
	})

	When("the self-signed issuer does not exist", func() {
		BeforeEach(func() {
			selfSignedIssuer := &certv1alpha1.Issuer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "kyma-system",
					Name:      "cfapi-self-signed-issuer",
				},
			}
			Expect(adminClient.Delete(ctx, selfSignedIssuer)).To(Succeed())
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})
})
