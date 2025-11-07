package values_test

import (
	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/installable/values"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("CFAPI Config", func() {
	var (
		cfAPIConfig  *values.CFAPIConfig
		instCfg      v1alpha1.InstallationConfig
		helmValues   map[string]any
		getValuesErr error
	)

	BeforeEach(func() {
		instCfg = v1alpha1.InstallationConfig{
			CFDomain:          "korifi.example.com",
			KorifiIngressHost: "cfapi.korifi.example.com",
			UAAURL:            "https://uaa.example.com",
			RootNamespace:     "my-root-ns",
			CFAdmins:          []string{"cf-admin@example.com"},
		}

		cfAPIConfig = values.NewCFAPIConfig()
	})

	JustBeforeEach(func() {
		helmValues, getValuesErr = cfAPIConfig.GetValues(ctx, instCfg)
	})

	It("returns helm values", func() {
		Expect(getValuesErr).NotTo(HaveOccurred())
		Expect(helmValues).To(MatchAllKeys(Keys{
			"cfDomain":          Equal("korifi.example.com"),
			"korifiIngressHost": Equal("cfapi.korifi.example.com"),
			"uaaUrl":            Equal("https://uaa.example.com"),
			"rootNamespace":     Equal("my-root-ns"),
			"cfapiAdmins":       ConsistOf(Equal("sap.ids:cf-admin@example.com")),
		}))
	})

	When("the korifi ingress host is not set", func() {
		BeforeEach(func() {
			instCfg.KorifiIngressHost = ""
		})

		It("returns an error", func() {
			Expect(getValuesErr).To(MatchError("korifi ingress host not available yet"))
		})
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
})
