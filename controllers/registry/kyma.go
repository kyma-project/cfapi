package registry

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const KymaRegistrySecret = "dockerregistry-config-external"

type Kyma struct {
	k8sClient client.Client
	scheme    *runtime.Scheme
}

func NewKyma(k8sClient client.Client, scheme *runtime.Scheme) *Kyma {
	return &Kyma{
		k8sClient: k8sClient,
		scheme:    scheme,
	}
}

func (k *Kyma) GetRegistrySecret(ctx context.Context, namespace string) (*corev1.Secret, error) {
	if !k.dockerRegistryModuleIsEnabled(ctx) {
		return nil, errors.New("dockerregistry kyma module is not enabled")
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      KymaRegistrySecret,
		},
	}

	err := k.k8sClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (k *Kyma) dockerRegistryModuleIsEnabled(ctx context.Context) bool {
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
