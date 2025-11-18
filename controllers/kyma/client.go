package kyma

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Client struct {
	ContainerRegistry *ContainerRegistry
	UAA               *UAA
	Users             *Users
	Gateway           *Gateway
}

func NewClient(k8sClient client.Client) *Client {
	return &Client{
		ContainerRegistry: NewContainerRegistry(k8sClient),
		UAA:               NewUAA(k8sClient),
		Users:             NewUsers(k8sClient),
		Gateway:           NewGateway(k8sClient),
	}
}
