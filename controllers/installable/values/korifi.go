package values

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/installable/values/secrets"
	"gopkg.in/yaml.v3"
)

type Korifi struct {
	registrySecret *secrets.Docker
}

func NewKorifi(registrySecret *secrets.Docker) *Korifi {
	return &Korifi{
		registrySecret: registrySecret,
	}
}

func (k *Korifi) GetValues(ctx context.Context, config v1alpha1.InstallationConfig) (map[string]any, error) {
	containerRegistryServer, err := k.getContainerRegistryServer(ctx, config.ContainerRegistrySecret)
	if err != nil {
		return nil, err
	}

	values := map[string]any{
		"adminUserName":                "cf-admin",
		"generateInternalCertificates": false,
		"containerRegistrySecrets":     []string{config.ContainerRegistrySecret},
		"containerRepositoryPrefix":    containerRegistryServer + "/",
		"defaultAppDomainName":         "apps." + config.CFDomain,
		"api": map[string]any{
			"apiServer": map[string]any{
				"url": "cfapi." + config.CFDomain,
			},
			"uaaURL": config.UAAURL,
		},
		"kpackImageBuilder": map[string]any{
			"builderRepository": containerRegistryServer + "/cfapi/kpack-builder",
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

func (k *Korifi) getContainerRegistryServer(ctx context.Context, registrySecretName string) (string, error) {
	registryConfig, err := k.registrySecret.GetRegistryConfig(ctx, "cfapi-system", registrySecretName)
	if err != nil {
		return "", fmt.Errorf("failed to get docker config from secret %s: %w", registrySecretName, err)
	}

	servers := slices.Collect(maps.Keys(registryConfig.Auths))
	if len(servers) == 0 {
		return "", fmt.Errorf("no registry server found in secret %s", registrySecretName)
	}

	return servers[0], nil
}

type NoValues struct{}

func (v NoValues) GetValues(ctx context.Context, config v1alpha1.InstallationConfig) (map[string]any, error) {
	return map[string]any{}, nil
}
