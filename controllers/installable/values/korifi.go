package values

import (
	"context"
	"fmt"

	"github.com/kyma-project/cfapi/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Korifi struct {
	k8sClient        client.Client
	releaseNamespace string
}

func NewKorifi(k8sClient client.Client, releaseNamespace string) *Korifi {
	return &Korifi{
		k8sClient:        k8sClient,
		releaseNamespace: releaseNamespace,
	}
}

func (k *Korifi) GetValues(ctx context.Context, config v1alpha1.InstallationConfig) (map[string]any, error) {
	if err := k.ensureCertificateSecrets(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure required certificate secrets: %w", err)
	}

	return map[string]any{
		"systemNamespace":              "cfapi-system",
		"adminUserName":                "cf-admin",
		"generateInternalCertificates": false,
		"containerRegistrySecrets":     []any{config.ContainerRegistrySecret},
		"containerRepositoryPrefix":    config.ContainerRepositoryPrefix,
		"defaultAppDomainName":         "apps." + config.CFDomain,
		"api": map[string]any{
			"apiServer": map[string]any{
				"url": "cfapi." + config.CFDomain,
			},
			"uaaURL": config.UAAURL,
		},
		"kpackImageBuilder": map[string]any{
			"builderRepository": config.BuilderRepository,
		},
		"networking": map[string]any{
			"gatewayNamespace": "cfapi-system",
			"gatewayClass":     config.GatewayType,
		},
		"experimental": map[string]any{
			"managedServices": map[string]any{
				"enabled": true,
			},
			"uaa": map[string]any{
				"enabled": true,
				"url":     config.UAAURL,
			},
		},
	}, nil
}

func (k *Korifi) ensureCertificateSecrets(ctx context.Context) error {
	for _, certSecret := range []string{
		"korifi-api-ingress-cert",
		"korifi-workloads-ingress-cert",
		"korifi-api-internal-cert",
		"korifi-controllers-webhook-cert",
	} {
		certSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: k.releaseNamespace,
				Name:      certSecret,
			},
		}
		if err := k.k8sClient.Get(ctx, client.ObjectKeyFromObject(certSecret), certSecret); err != nil {
			return err
		}
	}

	return nil
}
