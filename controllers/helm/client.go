package helm

import (
	"context"
	"fmt"
	"reflect"

	golog "log"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
)

type HelmResult struct {
	ReleaseStatus release.Status
	Message       string
}

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Apply(ctx context.Context, chartPath string, releaseNamespace string, releaseName string, values map[string]any) (HelmResult, error) {
	log := logr.FromContextOrDiscard(ctx).WithName("helm").WithValues("chart", releaseName)

	chart, err := loader.Load(chartPath)
	if err != nil {
		return HelmResult{}, fmt.Errorf("failed to load chart at %s: %w", chartPath, err)
	}

	latestRelease, err := getLatestReleases(releaseNamespace, releaseName)
	if err != nil {
		log.Error(err, "failed to get latest release")
		return HelmResult{}, fmt.Errorf("failed to get latest release %s in namespace %s: %w", releaseName, releaseNamespace, err)
	}

	if latestRelease == nil {
		return c.install(ctx, chart, releaseNamespace, releaseName, values)
	}

	if latestRelease.Info.Status.IsPending() {
		log.Info("helm operation is pending", "releaseStatus", latestRelease.Info.Status)
		return HelmResult{
			ReleaseStatus: latestRelease.Info.Status,
			Message:       "operation pending",
		}, nil
	}

	equalValues, err := equalValues(latestRelease.Config, values)
	if err != nil {
		return HelmResult{}, fmt.Errorf("failed to compare release values: %w", err)
	}
	equalVersions := chart.Metadata.Version == latestRelease.Chart.Metadata.Version

	if equalValues && equalVersions && latestRelease.Info.Status != release.StatusFailed {
		log.Info("helm chart does not need update")
		return HelmResult{
			ReleaseStatus: latestRelease.Info.Status,
		}, nil
	}

	return c.upgrade(ctx, chart, releaseNamespace, releaseName, values)
}

func (c *Client) Uninstall(ctx context.Context, releaseNamespace string, releaseName string) (HelmResult, error) {
	log := logr.FromContextOrDiscard(ctx).WithName("helm-delete").WithValues("chart", releaseName)
	log.Info("starting delete")

	latestRelease, err := getLatestReleases(releaseNamespace, releaseName)
	if err != nil {
		log.Error(err, "failed to get latest release")
		return HelmResult{}, fmt.Errorf("failed to get latest release %s in namespace %s: %w", releaseName, releaseNamespace, err)
	}

	if latestRelease == nil {
		log.Info("release not found, nothing to uninstall")
		return HelmResult{}, nil
	}

	if latestRelease.Info.Status == release.StatusUninstalling {
		log.Info("uninstall operation is ongoing", "releaseStatus", latestRelease.Info.Status)
		return HelmResult{
			ReleaseStatus: latestRelease.Info.Status,
			Message:       "operation pending",
		}, nil
	}

	actionConfig, err := newHelmActionConfig(releaseNamespace)
	if err != nil {
		return HelmResult{}, fmt.Errorf("failed to init helm action config: %w", err)
	}

	uninstallAction := action.NewUninstall(actionConfig)
	uninstallAction.IgnoreNotFound = true

	uninstResult, err := uninstallAction.Run(releaseName)
	if err != nil {
		return HelmResult{
			ReleaseStatus: release.StatusUnknown,
			Message:       err.Error(),
		}, nil
	}

	helmResult := HelmResult{}
	if uninstResult != nil {
		if uninstResult.Release != nil && uninstResult.Release.Info != nil {
			helmResult.ReleaseStatus = uninstResult.Release.Info.Status
		}
		helmResult.Message = uninstResult.Info
	}

	return helmResult, nil
}

func equalValues(values1, values2 map[string]any) (bool, error) {
	v1, err := marshalUnmarshal(values1)
	if err != nil {
		return false, fmt.Errorf("failed to marshal-unmarshal values1: %w", err)
	}
	v2, err := marshalUnmarshal(values2)
	if err != nil {
		return false, fmt.Errorf("failed to marshal-unmarshal values2: %w", err)
	}

	return reflect.DeepEqual(v1, v2), nil
}

func marshalUnmarshal(values map[string]any) (map[string]any, error) {
	valuesBytes, err := yaml.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal values: %w", err)
	}

	valuesUnamrshalled := make(map[string]any)
	err = yaml.Unmarshal(valuesBytes, &valuesUnamrshalled)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal values: %w", err)
	}

	return valuesUnamrshalled, nil
}

func getLatestReleases(releaseNamespace string, releaseName string) (*release.Release, error) {
	actionConfig, err := newHelmActionConfig(releaseNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to init helm action config: %w", err)
	}

	listClient := action.NewList(actionConfig)
	listClient.Sort = action.ByDateDesc
	listClient.Limit = 1
	listClient.Filter = fmt.Sprintf("^%s$", releaseName)

	versions, err := listClient.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to list helm release %s%s: %w", releaseNamespace, releaseName, err)
	}

	if len(versions) == 0 {
		return nil, nil
	}

	return versions[0], nil
}

func (c *Client) install(ctx context.Context, installedChart *chart.Chart, releaseNamespace string, releaseName string, values map[string]any) (HelmResult, error) {
	log := logr.FromContextOrDiscard(ctx).WithName("helm-install").WithValues("chart", releaseName)
	log.Info("starting install")

	actionConfig, err := newHelmActionConfig(releaseNamespace)
	if err != nil {
		return HelmResult{}, fmt.Errorf("failed to init helm action config: %w", err)
	}

	installAction := action.NewInstall(actionConfig)
	installAction.Namespace = releaseNamespace
	installAction.CreateNamespace = true
	installAction.ReleaseName = releaseName

	rel, err := installAction.Run(installedChart, values)
	if err != nil {
		return HelmResult{
			ReleaseStatus: release.StatusUnknown,
			Message:       err.Error(),
		}, nil
	}

	return HelmResult{
		ReleaseStatus: rel.Info.Status,
	}, nil
}

func (c *Client) upgrade(ctx context.Context, upgradedChart *chart.Chart, releaseNamespace string, releaseName string, values map[string]any) (HelmResult, error) {
	log := logr.FromContextOrDiscard(ctx).WithName("helm-upgrade").WithValues("chart", releaseName)
	log.Info("starting upgrade")

	actionConfig, err := newHelmActionConfig(releaseNamespace)
	if err != nil {
		return HelmResult{}, fmt.Errorf("failed to init helm action config: %w", err)
	}

	upgradeAction := action.NewUpgrade(actionConfig)
	upgradeAction.Namespace = releaseNamespace
	upgradeAction.Install = true

	rel, err := upgradeAction.Run(releaseName, upgradedChart, values)
	if err != nil {
		return HelmResult{
			ReleaseStatus: release.StatusUnknown,
			Message:       err.Error(),
		}, nil
	}

	return HelmResult{
		ReleaseStatus: rel.Info.Status,
	}, nil
}

func newHelmActionConfig(releaseNamespace string) (*action.Configuration, error) {
	helmSettings := cli.New()
	actionConfig := new(action.Configuration)
	err := actionConfig.Init(helmSettings.RESTClientGetter(), releaseNamespace, "secret", golog.Printf)
	if err != nil {
		return nil, fmt.Errorf("failed to init helm action config: %w", err)
	}

	return actionConfig, nil
}
