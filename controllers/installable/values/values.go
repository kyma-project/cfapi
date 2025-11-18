package values

import (
	"context"

	"github.com/kyma-project/cfapi/api/v1alpha1"
)

type Override map[string]any

func (v Override) GetValues(ctx context.Context, config v1alpha1.InstallationConfig) (map[string]any, error) {
	return v, nil
}
