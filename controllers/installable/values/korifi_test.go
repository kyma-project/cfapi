package values_test

import (
	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/installable/values"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Korifi", func() {
	var (
		korifi       *values.Korifi
		instCfg      v1alpha1.InstallationConfig
		helmValues   map[string]any
		getValuesErr error
	)

	BeforeEach(func() {
		instCfg = v1alpha1.InstallationConfig{
			RootNamespace:           "my-root-ns",
			ContainerRegistrySecret: "my-registry-secret",
			ContainerRegistryURL:    "my-registry.com",
			UAAURL:                  "https://uaa.example.com",
			CFDomain:                "korifi.example.com",
		}

		korifi = values.NewKorifi()
	})

	JustBeforeEach(func() {
		helmValues, getValuesErr = korifi.GetValues(ctx, instCfg)
	})

	It("returns helm values", func() {
		Expect(getValuesErr).NotTo(HaveOccurred())
		Expect(helmValues).To(Equal(map[string]any{
			"adminUserName":                "cf-admin",
			"generateInternalCertificates": false,
			"containerRegistrySecrets":     []any{"my-registry-secret"},
			"containerRepositoryPrefix":    "my-registry.com" + "/",
			"defaultAppDomainName":         "apps.korifi.example.com",
			"api": map[string]any{
				"apiServer": map[string]any{
					"url": "cfapi." + instCfg.CFDomain,
				},
				"uaaURL": instCfg.UAAURL,
			},
			"kpackImageBuilder": map[string]any{
				"builderRepository": "my-registry.com/cfapi/kpack-builder",
			},
			"networking": map[string]any{
				"gatewayClass": "istio",
			},
			"experimental": map[string]any{
				"managedServices": map[string]any{
					"enabled": true,
				},
				"uaa": map[string]any{
					"enabled": true,
					"url":     "https://uaa.example.com",
				},
			},
		}))
	})
})
