package values

import (
	"context"

	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/kyma"
)

type Prerequisites struct{}

func NewPrerequisites() *Prerequisites {
	return &Prerequisites{}
}

func (k *Prerequisites) GetValues(ctx context.Context, config v1alpha1.InstallationConfig) (map[string]any, error) {
	propagationEnabled := config.ContainerRegistrySecret != kyma.ContainerRegistrySecretName
	propagationConfig := map[string]any{
		"enabled": propagationEnabled,
	}
	if propagationEnabled {
		propagationConfig["sourceNamespace"] = "cfapi-system"
		propagationConfig["destinationNamespace"] = config.RootNamespace
	}

	return map[string]any{
		"cfDomain":                  config.CFDomain,
		"useSelfSignedCertificates": config.UseSelfSignedCertificates,
		"containerRegistrySecret": map[string]any{
			"name":        config.ContainerRegistrySecret,
			"propagation": propagationConfig,
		},
	}, nil
}
