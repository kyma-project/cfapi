package secrets

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DockerRegistryConfig struct {
	Auths map[string]DockerRegistryAuth `json:"auths"`
}

type DockerRegistryAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Docker struct {
	k8sClient client.Client
}

func NewDocker(k8sClient client.Client) *Docker {
	return &Docker{
		k8sClient: k8sClient,
	}
}

func (d *Docker) GetRegistryConfig(ctx context.Context, secretNamespace string, secretName string) (DockerRegistryConfig, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: secretNamespace,
			Name:      secretName,
		},
	}

	err := d.k8sClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
	if err != nil {
		return DockerRegistryConfig{}, fmt.Errorf("failed to get docker registry secret %s/%s: %w", secretNamespace, secretName, err)
	}

	config := &DockerRegistryConfig{}
	err = json.Unmarshal(secret.Data[corev1.DockerConfigJsonKey], config)
	if err != nil {
		return DockerRegistryConfig{}, fmt.Errorf("failed to unmarshal docker registry config from secret %s/%s: %w", secretNamespace, secretName, err)
	}

	return *config, nil
}
