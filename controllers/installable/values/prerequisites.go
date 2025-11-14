package values

import (
	"context"
	"fmt"

	certv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/kyma"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const selfSignedIssuerName = "cfapi-self-signed-issuer"

type Prerequisites struct {
	k8sClient client.Client
}

func NewPrerequisites(k8sClient client.Client) *Prerequisites {
	return &Prerequisites{
		k8sClient: k8sClient,
	}
}

func (p *Prerequisites) GetValues(ctx context.Context, config v1alpha1.InstallationConfig) (map[string]any, error) {
	systemNamespace := "kyma-system"

	if err := p.ensureSelfSignedIssuer(ctx, systemNamespace); err != nil {
		return nil, fmt.Errorf("failed to get self seigned issuer: %w", err)
	}

	propagationEnabled := config.ContainerRegistrySecret != kyma.ContainerRegistrySecretName
	if config.DisableContainerRegistrySecretPropagation {
		propagationEnabled = false
	}

	propagationConfig := map[string]any{
		"enabled": propagationEnabled,
	}
	if propagationEnabled {
		propagationConfig["sourceNamespace"] = "cfapi-system"
		propagationConfig["destinationNamespace"] = config.RootNamespace
	}

	return map[string]any{
		"systemNamespace":           systemNamespace,
		"useSelfSignedCertificates": config.UseSelfSignedCertificates,
		"selfSignedIssuer":          selfSignedIssuerName,
		"cfDomain":                  config.CFDomain,
		"gatewayType":               config.GatewayType,
		"containerRegistrySecret": map[string]any{
			"name":        config.ContainerRegistrySecret,
			"propagation": propagationConfig,
		},
	}, nil
}

func (p *Prerequisites) ensureSelfSignedIssuer(ctx context.Context, systemNamespace string) error {
	selfSignedIssuer := certv1alpha1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: systemNamespace,
			Name:      selfSignedIssuerName,
		},
	}
	return p.k8sClient.Get(ctx, client.ObjectKeyFromObject(&selfSignedIssuer), &selfSignedIssuer)
}
