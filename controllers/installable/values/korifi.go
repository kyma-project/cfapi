package values

import (
	"context"

	"github.com/kyma-project/cfapi/api/v1alpha1"
)

type Korifi struct{}

func NewKorifi() *Korifi {
	return &Korifi{}
}

func (k *Korifi) GetValues(ctx context.Context, config v1alpha1.InstallationConfig) (map[string]any, error) {
	return map[string]any{
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
			"gatewayClass": "istio",
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

type NoValues struct{}

func (v NoValues) GetValues(ctx context.Context, config v1alpha1.InstallationConfig) (map[string]any, error) {
	return map[string]any{}, nil
}
