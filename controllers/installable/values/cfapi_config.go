package values

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/BooleanCat/go-functional/v2/it"
	"github.com/kyma-project/cfapi/api/v1alpha1"
)

type CFAPIConfig struct{}

func NewCFAPIConfig() *CFAPIConfig {
	return &CFAPIConfig{}
}

func (k *CFAPIConfig) GetValues(ctx context.Context, config v1alpha1.InstallationConfig) (map[string]any, error) {
	if config.KorifiIngressHost == "" {
		return nil, errors.New("korifi ingress host not available yet")
	}

	return map[string]any{
		"cfDomain":          config.CFDomain,
		"korifiIngressHost": config.KorifiIngressHost,
		"uaaUrl":            config.UAAURL,
		"rootNamespace":     config.RootNamespace,
		"cfapiAdmins":       slices.Collect(it.Map(slices.Values(config.CFAdmins), withOIDCPrefix)),
	}, nil
}

func withOIDCPrefix(user string) any {
	if !strings.HasPrefix(user, "sap.ids:") {
		return "sap.ids:" + user
	}

	return user
}
