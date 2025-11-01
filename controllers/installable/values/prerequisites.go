package values

import (
	"context"
	"fmt"

	"github.com/kyma-project/cfapi/api/v1alpha1"
	"gopkg.in/yaml.v3"
)

type Prerequisites struct{}

func NewPrerequisites() *Prerequisites {
	return &Prerequisites{}
}

func (k *Prerequisites) GetValues(ctx context.Context, config v1alpha1.InstallationConfig) (map[string]any, error) {
	values := map[string]any{
		"sourceNamespace":      "cfapi-system",
		"sourceSecret":         config.ContainerRegistrySecret,
		"destinationNamespace": config.RootNamespace,
		"destinationSecret":    config.ContainerRegistrySecret,
	}

	valuesBytes, err := yaml.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal values: %w", err)
	}

	values = map[string]any{}
	err = yaml.Unmarshal(valuesBytes, &values)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal values bytes: %w", err)
	}

	return values, nil
}
