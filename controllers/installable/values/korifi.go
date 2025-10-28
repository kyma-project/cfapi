package values

import (
	"context"
	"fmt"

	"github.com/kyma-project/cfapi/api/v1alpha1"
	"gopkg.in/yaml.v3"
)

type Korifi struct{}

func NewKorifi() *Korifi {
	return &Korifi{}
}

func (k *Korifi) GetValues(ctx context.Context, config v1alpha1.InstallationConfig) (map[string]any, error) {
	values := map[string]any{
		"adminUserName":                "cf-admin",
		"generateInternalCertificates": false,
		"containerRegistrySecrets":     []string{config.ContainerRegistrySecret},
		"containerRepositoryPrefix":    config.ContainerRegistryURL + "/",
		"defaultAppDomainName":         "apps." + config.CFDomain,
		"api": map[string]any{
			"apiServer": map[string]any{
				"url": "cfapi." + config.CFDomain,
			},
			"uaaURL": config.UAAURL,
		},
		"kpackImageBuilder": map[string]any{
			"builderRepository": config.ContainerRegistryURL + "/cfapi/kpack-builder",
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
	}

	valuesBytes, err := yaml.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal values: %w", err)
	}

	values = map[string]any{}
	err = yaml.Unmarshal(valuesBytes, &values)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal values bytes: %w", err)
	}

	return values, nil
}

type NoValues struct{}

func (v NoValues) GetValues(ctx context.Context, config v1alpha1.InstallationConfig) (map[string]any, error) {
	return map[string]any{}, nil
}
