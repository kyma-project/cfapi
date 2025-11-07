package values_test

import (
	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/installable/values"
	"github.com/kyma-project/cfapi/controllers/kyma"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Prerequisites", func() {
	var (
		prerequisites *values.Prerequisites
		instCfg       v1alpha1.InstallationConfig
		helmValues    map[string]any
	)

	BeforeEach(func() {
		instCfg = v1alpha1.InstallationConfig{
			CFDomain:                  "korifi.example.com",
			UseSelfSignedCertificates: true,
			ContainerRegistrySecret:   kyma.ContainerRegistrySecretName,
			RootNamespace:             "my-root-ns",
		}

		prerequisites = values.NewPrerequisites()
	})

	JustBeforeEach(func() {
		var err error
		helmValues, err = prerequisites.GetValues(ctx, instCfg)
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns helm values", func() {
		Expect(helmValues).To(MatchAllKeys(Keys{
			"cfDomain":                  Equal("korifi.example.com"),
			"useSelfSignedCertificates": Equal(true),
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
	})
})
