package values_test

import (
	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/installable/values"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Korifi", func() {
	var (
		korifi     *values.Korifi
		instCfg    v1alpha1.InstallationConfig
		helmValues map[string]any
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

		korifi = values.NewKorifi()
	})

	JustBeforeEach(func() {
		var err error
		helmValues, err = korifi.GetValues(ctx, instCfg)
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns helm values", func() {
		Expect(helmValues).To(MatchAllKeys(Keys{
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
})
