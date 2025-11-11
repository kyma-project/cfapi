package kyma

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const ContainerRegistrySecretName = "dockerregistry-config-external"

type ContainerRegistry struct {
	k8sClient client.Client
}

func NewContainerRegistry(k8sClient client.Client) *ContainerRegistry {
	return &ContainerRegistry{
		k8sClient: k8sClient,
	}
}

func (k *ContainerRegistry) GetRegistrySecret(ctx context.Context, namespace string) (*corev1.Secret, error) {
	if !k.dockerRegistryModuleIsEnabled(ctx) {
		return nil, errors.New("dockerregistry kyma module is not enabled")
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      ContainerRegistrySecretName,
		},
	}

	err := k.k8sClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
	if err != nil {
		return nil, fmt.Errorf("could not get the kyma docker container registry external secret: %w. Make sure a docker registry resource exists and has its external access is enabled", err)
	}

	return secret, nil
}

func (k *ContainerRegistry) GetRegistryURL(ctx context.Context, namespace string) (string, error) {
	registrySecret, err := k.GetRegistrySecret(ctx, namespace)
	if err != nil {
		return "", err
	}
	registryURL, ok := registrySecret.Data["pushRegAddr"]
	if !ok {
		return "", fmt.Errorf("registry secret %s/%s is missing the pushRegAddr key", registrySecret.Namespace, registrySecret.Name)
	}
	return string(registryURL), nil
}

func (k *ContainerRegistry) dockerRegistryModuleIsEnabled(ctx context.Context) bool {
	logger := log.FromContext(ctx)

	crds := &v1.CustomResourceDefinitionList{}
	err := k.k8sClient.List(ctx, crds)
	if err != nil {
		logger.Error(err, "failed to list CRDs")
		return false
	}

	for _, i := range crds.Items {
		if i.Spec.Names.Kind == "DockerRegistry" {
			return true
		}
	}

	return false
}
