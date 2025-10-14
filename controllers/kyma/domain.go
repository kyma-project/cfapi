package kyma

import (
	"context"
	"errors"
	"fmt"

	istioclient "istio.io/client-go/pkg/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Domain struct {
	istioClient istioclient.Interface
}

func NewDomain(istioClient istioclient.Interface) *Domain {
	return &Domain{
		istioClient: istioClient,
	}
}

func (d *Domain) Get(ctx context.Context) (string, error) {
	kymaGw, err := d.istioClient.NetworkingV1beta1().Gateways("kyma-system").Get(ctx, "kyma-gateway", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get the kyma system gateway: %w", err)
	}

	if len(kymaGw.Spec.Servers) == 0 {
		return "", errors.New("failed to get the kyma gateway domain: gateway has no servers")
	}

	wildCardDomain := kymaGw.Spec.Servers[0].Hosts[0]
	return wildCardDomain[2:], nil
}
