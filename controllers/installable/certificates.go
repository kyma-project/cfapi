package installable

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/tools"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Certificates struct {
	k8sClient       client.Client
	yamlInstallable Installable
}

func NewCertificates(k8sClient client.Client, yamlInstallable Installable) *Certificates {
	return &Certificates{
		k8sClient:       k8sClient,
		yamlInstallable: yamlInstallable,
	}
}

func (c *Certificates) Install(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder EventRecorder) (Result, error) {
	log := logr.FromContextOrDiscard(ctx).WithName("certificates")
	result, err := c.yamlInstallable.Install(ctx, config, eventRecorder)
	if err != nil {
		return Result{}, tools.LogAndReturn(log, fmt.Errorf("failed to install certificates: %w", err))
	}

	if result.State != ResultStateSuccess {
		return result, nil
	}

	expectedSecrets := []string{
		"korifi-api-ingress-cert",
		"korifi-api-internal-cert",
		"korifi-workloads-ingress-cert",
		"korifi-controllers-webhook-cert",
		"korifi-kpack-image-builder-webhook-cert",
		"korifi-statefulset-runner-webhook-cert",
	}

	err = c.ensureSecretsExist(ctx, expectedSecrets)
	if err != nil {
		log.Info(fmt.Sprintf("certificates not yet installed: %s", err.Error()))
		eventRecorder.Event(EventNormal, "CertificatesInstallation", fmt.Sprintf("Certificates not yet installed: %s", err.Error()))
		return Result{
			State:   ResultStateInProgress,
			Message: fmt.Sprintf("Certificates being installed: %s", err.Error()),
		}, nil
	}

	eventRecorder.Event(EventNormal, "CertificatesInstallation", "Certificates installed successfully")
	return Result{
		State:   ResultStateSuccess,
		Message: "Certificates installed successfully",
	}, nil
}

func (c *Certificates) ensureSecretsExist(ctx context.Context, expectedSecrets []string) error {
	var result *multierror.Error

	for _, secret := range expectedSecrets {
		if err := c.ensureSecretExist(ctx, secret); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}

func (c *Certificates) ensureSecretExist(ctx context.Context, secretName string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "korifi",
			Name:      secretName,
		},
	}

	return c.k8sClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
}
