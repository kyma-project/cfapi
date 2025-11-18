package kyma

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/istio/operator/api/v1alpha2"
	networkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Gateway struct {
	k8sClient client.Client
}

func NewGateway(k8sClient client.Client) *Gateway {
	return &Gateway{
		k8sClient: k8sClient,
	}
}

func (g *Gateway) KorifiGatewayType(cfAPI *v1alpha1.CFAPI) string {
	if cfAPI.Spec.GatewayType == "" {
		return v1alpha1.GatewayTypeContour
	}
	return cfAPI.Spec.GatewayType
}

func (g *Gateway) Validate(ctx context.Context, cfAPI *v1alpha1.CFAPI) error {
	if cfAPI.Spec.GatewayType == v1alpha1.GatewayTypeIstio {
		aphaGWAPIEnabled, err := g.isAplhaGatewayAPIEnabled(ctx)
		if err != nil {
			return err
		}

		if !aphaGWAPIEnabled {
			return errors.New("alpha gateway API feature is not enabled in istio. To fix this, enable the `experimental` channel on the istio module and set `spec.experimental.pilot.enableAlphaGatewayAPI` to `true` on the `kyma-system/default` Istio resource")
		}
	}

	return nil
}

func (g *Gateway) isAplhaGatewayAPIEnabled(ctx context.Context) (bool, error) {
	istio := v1alpha2.Istio{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kyma-system",
			Name:      "default",
		},
	}

	err := g.k8sClient.Get(ctx, client.ObjectKeyFromObject(&istio), &istio)
	if err != nil {
		return false, fmt.Errorf("failed to get the istio resource: %w. Make sure the istio kyma module is enabled", err)
	}

	if istio.Spec.Experimental == nil {
		return false, nil
	}

	return istio.Spec.Experimental.EnableAlphaGatewayAPI, nil
}

func (g *Gateway) KymaDomain(ctx context.Context) (string, error) {
	istioGateway := &networkingv1beta1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kyma-system",
			Name:      "kyma-gateway",
		},
	}
	if err := g.k8sClient.Get(ctx, client.ObjectKeyFromObject(istioGateway), istioGateway); err != nil {
		return "", fmt.Errorf("failed to get the kyma system gateway: %w", err)
	}

	if len(istioGateway.Spec.Servers) == 0 {
		return "", errors.New("failed to get the kyma gateway domain: gateway has no servers")
	}

	wildCardDomain := istioGateway.Spec.Servers[0].Hosts[0]
	return wildCardDomain[2:], nil
}

func (g *Gateway) KorifiIngressService(cfAPI *v1alpha1.CFAPI) string {
	if cfAPI.Spec.GatewayType == v1alpha1.GatewayTypeIstio {
		return "korifi-istio"
	}
	return "contour-envoy"
}
