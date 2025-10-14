package values_test

import (
	"encoding/json"

	v1alpha1 "github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/installable/values"
	"github.com/kyma-project/cfapi/controllers/installable/values/secrets"
	"github.com/kyma-project/cfapi/tools/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Korifi", func() {
	var (
		korifi       *values.Korifi
		instCfg      v1alpha1.InstallationConfig
		helmValues   map[string]any
		getValuesErr error
	)

	BeforeEach(func() {
		config := secrets.DockerRegistryConfig{
			Auths: map[string]secrets.DockerRegistryAuth{
				"my-registry.com": {},
			},
		}
		configBytes, err := json.Marshal(config)
		Expect(err).NotTo(HaveOccurred())

		registrySecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "cfapi-system",
				Name:      "my-registry-secret",
			},
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: configBytes,
			},
		}
		Expect(adminClient.Create(ctx, registrySecret)).To(Succeed())

		instCfg = v1alpha1.InstallationConfig{
			RootNamespace:           "my-root-ns",
			ContainerRegistrySecret: registrySecret.Name,
			UAAURL:                  "https://uaa.example.com",
			CFDomain:                "korifi.example.com",
		}

		korifi = values.NewKorifi(secrets.NewDocker(adminClient))
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

	When("reading the container registry secret fails", func() {
		BeforeEach(func() {
			instCfg.ContainerRegistrySecret = "non-existent"
		})

		It("returns an error", func() {
			Expect(getValuesErr).To(MatchError(ContainSubstring("failed to get docker config from secret")))
		})
	})

	When("the container registry secret does not specify servers", func() {
		BeforeEach(func() {
			registrySecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "cfapi-system",
					Name:      "my-registry-secret",
				},
			}
			Expect(adminClient.Get(ctx, client.ObjectKeyFromObject(registrySecret), registrySecret)).To(Succeed())
			Expect(k8s.Patch(ctx, adminClient, registrySecret, func() {
				registrySecret.Data = map[string][]byte{
					corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`),
				}
			})).To(Succeed())
		})

		It("returns an error", func() {
			Expect(getValuesErr).To(MatchError(ContainSubstring("no registry server found")))
		})
	})
})
