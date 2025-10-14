package kyma

import (
	"context"
	"fmt"

	"github.com/kyma-project/istio/operator/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Istio struct {
	k8sClient client.Client
}

func NewIstio(k8sClient client.Client) *Istio {
	return &Istio{
		k8sClient: k8sClient,
	}
}

func (i *Istio) IsAplhaGatewayAPIEnabled(ctx context.Context) (bool, error) {
	istio := v1alpha2.Istio{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kyma-system",
			Name:      "default",
		},
	}

	err := i.k8sClient.Get(ctx, client.ObjectKeyFromObject(&istio), &istio)
	if err != nil {
		return false, fmt.Errorf("failed to get the istio resource: %w", err)
	}

	if istio.Spec.Experimental == nil {
		return false, nil
	}

	return istio.Spec.Experimental.EnableAlphaGatewayAPI, nil
}
