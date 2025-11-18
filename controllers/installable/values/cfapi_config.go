package values

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/BooleanCat/go-functional/v2/it"
	"github.com/kyma-project/cfapi/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CFAPIConfig struct {
	k8sClient client.Client
}

func NewCFAPIConfig(client client.Client) *CFAPIConfig {
	return &CFAPIConfig{
		k8sClient: client,
	}
}

func (k *CFAPIConfig) GetValues(ctx context.Context, config v1alpha1.InstallationConfig) (map[string]any, error) {
	korifiIngressHost, err := k.getKorifiIngressHost(ctx, config.KorifiIngressService)
	if err != nil {
		return nil, fmt.Errorf("failed to get korifi ingress host: %w", err)
	}

	return map[string]any{
		"cfDomain":          config.CFDomain,
		"korifiIngressHost": korifiIngressHost,
		"uaaUrl":            config.UAAURL,
		"rootNamespace":     config.RootNamespace,
		"cfapiAdmins":       slices.Collect(it.Map(slices.Values(config.CFAdmins), withOIDCPrefix)),
	}, nil
}

func (k *CFAPIConfig) getKorifiIngressHost(ctx context.Context, korifiIngressServiceName string) (string, error) {
	korifiIngressService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "cfapi-system",
			Name:      korifiIngressServiceName,
		},
	}

	err := k.k8sClient.Get(ctx, client.ObjectKeyFromObject(korifiIngressService), korifiIngressService)
	if err != nil {
		return "", fmt.Errorf("failed to get korifi ingress service: %w", err)
	}

	if len(korifiIngressService.Status.LoadBalancer.Ingress) == 0 {
		return "", errors.New("korifi ingress service does not have an ingress assigned yet")
	}

	hostname := korifiIngressService.Status.LoadBalancer.Ingress[0].Hostname
	if hostname == "" {
		hostname = korifiIngressService.Status.LoadBalancer.Ingress[0].IP
	}

	return hostname, nil
}

func withOIDCPrefix(user string) any {
	if !strings.HasPrefix(user, "sap.ids:") {
		return "sap.ids:" + user
	}

	return user
}
