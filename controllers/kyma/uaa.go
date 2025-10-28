package kyma

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UAA struct {
	k8sClient client.Client
}

func NewUAA(k8sClient client.Client) *UAA {
	return &UAA{
		k8sClient: k8sClient,
	}
}

func (o *UAA) GetURL(ctx context.Context) (string, error) {
	btpServiceOperatorSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kyma-system",
			Name:      "sap-btp-service-operator",
		},
	}
	err := o.k8sClient.Get(context.Background(), client.ObjectKeyFromObject(btpServiceOperatorSecret), btpServiceOperatorSecret)
	if err != nil {
		return "", fmt.Errorf("failed to get the btp service operator secret: %w. Make sure the btp operator kyma module is enbled", err)
	}

	tokenURLBytes, ok := btpServiceOperatorSecret.Data["tokenurl"]
	if !ok {
		return "", errors.New("btp service operator secret does not contain key 'tokenurl'")
	}

	return extractUaaURLFromTokenUrl(string(tokenURLBytes)), nil
}

func extractUaaURLFromTokenUrl(tokenUrl string) string {
	// input => https://worker1-q3zjpctt.authentication.eu12.hana.ondemand.com
	// output => https://uaa.cf.eu12.hana.ondemand.com

	parts := strings.Split(tokenUrl, ".")
	parts = parts[2:]

	return "https://uaa.cf." + strings.Join(parts, ".")
}
