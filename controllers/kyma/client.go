package kyma

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	istioclient "istio.io/client-go/pkg/clientset/versioned"
)

type Client struct {
	Domain            *Domain
	ContainerRegistry *ContainerRegistry
	UAA               *UAA
	Users             *Users
	Istio             *Istio
}

func NewClient(k8sClient client.Client, istioClient istioclient.Interface) *Client {
	return &Client{
		Domain:            NewDomain(istioClient),
		ContainerRegistry: NewContainerRegistry(k8sClient),
		UAA:               NewUAA(k8sClient),
		Users:             NewUsers(k8sClient),
		Istio:             NewIstio(k8sClient),
	}
}
