package installable

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/kyma-project/cfapi/api/v1alpha1"
	"github.com/kyma-project/cfapi/controllers/helm"
	"helm.sh/helm/v3/pkg/release"
)

type HelmClient interface {
	Apply(ctx context.Context, chartPath, namespace, name string, values map[string]any) (helm.HelmResult, error)
	Uninstall(ctx context.Context, namespace, name string) (helm.HelmResult, error)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate -o fake -fake-name HelmValuesProvider . HelmValuesProvider
type HelmValuesProvider interface {
	GetValues(ctx context.Context, config v1alpha1.InstallationConfig) (map[string]any, error)
}

type HelmChart struct {
	chartPath      string
	namespace      string
	name           string
	valuesProvider HelmValuesProvider
	helmClient     HelmClient
}

func NewHelmChart(chartPath string, namespace, name string, valuesProvider HelmValuesProvider, helmClient HelmClient) *HelmChart {
	return &HelmChart{
		helmClient:     helmClient,
		chartPath:      chartPath,
		namespace:      namespace,
		name:           name,
		valuesProvider: valuesProvider,
	}
}

func (h *HelmChart) Name() string {
	return fmt.Sprintf("Helm Installable: %s", h.name)
}

func (h *HelmChart) Install(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder EventRecorder) (Result, error) {
	log := logr.FromContextOrDiscard(ctx).WithName("helm").WithValues("chart", h.name)
	values, err := h.valuesProvider.GetValues(ctx, config)
	if err != nil {
		log.Error(err, "failed to get helm chart values")
		return Result{
			State:   ResultStateInProgress,
			Message: fmt.Sprintf("failed to get helm chart %s values: %s", h.name, err.Error()),
		}, nil
	}

	helmResult, err := h.helmClient.Apply(ctx, h.chartPath, h.namespace, h.name, values)
	if err != nil {
		log.Error(err, "failed to apply chart")
		return Result{
			State:   ResultStateFailed,
			Message: fmt.Sprintf("failed to install/upgrade helm chart %s: %s", h.name, err.Error()),
		}, nil
	}
	eventRecorder.Event(EventNormal, "HelmChartApplied", fmt.Sprintf("Helm chart %s applied with status %s", h.name, helmResult.ReleaseStatus))

	switch helmResult.ReleaseStatus {
	case release.StatusDeployed:
		eventRecorder.Event(EventNormal, "HelmChartDeployed", fmt.Sprintf("Helm chart %s deployed successfully", h.name))
		return Result{
			State: ResultStateSuccess,
		}, nil
	case release.StatusFailed:
		eventRecorder.Event(EventWarning, "HelmChartDeploymentFailed", fmt.Sprintf("Helm chart %s failed to deploy: %s", h.name, helmResult.Message))
		return Result{
			State:   ResultStateFailed,
			Message: helmResult.Message,
		}, nil
	default:
		eventRecorder.Event(EventNormal, "HelmChartDeploying", fmt.Sprintf("Helm chart %s is being deployed", h.name))
		return Result{
			State:   ResultStateInProgress,
			Message: fmt.Sprintf("helm chart %s is in status %s: %s", h.name, helmResult.ReleaseStatus, helmResult.Message),
		}, nil
	}
}

func (h *HelmChart) Uninstall(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder EventRecorder) (Result, error) {
	log := logr.FromContextOrDiscard(ctx).WithName("helm").WithValues("chart", h.name)

	helmResult, err := h.helmClient.Uninstall(ctx, h.namespace, h.name)
	if err != nil {
		log.Error(err, "failed to uninstall chart")
		return Result{
			State:   ResultStateFailed,
			Message: fmt.Sprintf("failed to uninstall helm chart %s: %s", h.name, err.Error()),
		}, nil
	}
	eventRecorder.Event(EventNormal, "HelmChartUninstalled", fmt.Sprintf("Helm chart %s uninstalled with status %s", h.name, helmResult.ReleaseStatus))

	if reflect.ValueOf(helmResult).IsZero() {
		eventRecorder.Event(EventNormal, "HelmChartUninstalled", fmt.Sprintf("Helm chart %s uninstalled successfully", h.name))
		return Result{
			State: ResultStateSuccess,
		}, nil
	}

	eventRecorder.Event(EventNormal, "HelmChartUninstalling", fmt.Sprintf("Helm chart %s is being uninstalled", h.name))
	return Result{
		State:   ResultStateInProgress,
		Message: fmt.Sprintf("helm chart %s is in status %s: %s", h.name, helmResult.ReleaseStatus, helmResult.Message),
	}, nil
}
