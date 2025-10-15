package kyma

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	istioclient "istio.io/client-go/pkg/clientset/versioned"
)

type Client struct {
	Domain            *Domain
	ContainerRegistry *ContainerRegistry
	UAA               *UAA
}

func NewClient(k8sClient client.Client, istioClient istioclient.Interface) *Client {
	return &Client{
		Domain:            NewDomain(istioClient),
		ContainerRegistry: NewContainerRegistry(k8sClient),
		UAA:               NewUAA(k8sClient),
	}
}
