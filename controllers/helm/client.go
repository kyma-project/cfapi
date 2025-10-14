package helm

import (
	"context"
	"fmt"

	golog "log"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
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

	releaseExists, releaseStatus, err := releaseExists(releaseNamespace, releaseName)
	if err != nil {
		return HelmResult{}, fmt.Errorf("failed to check if release %s exists in namespace %s: %w", releaseName, releaseNamespace, err)
	}

	if !releaseExists {
		return c.install(ctx, chartPath, releaseNamespace, releaseName, values)
	}

	if releaseStatus.IsPending() {
		log.Info("helm operation is pending", "releaseStatus", releaseStatus)
		return HelmResult{
			ReleaseStatus: releaseStatus,
			Message:       "operation pending",
		}, nil
	}

	return c.upgrade(ctx, chartPath, releaseNamespace, releaseName, values)
}

func releaseExists(namespace, name string) (bool, release.Status, error) {
	actionConfig, err := newHelmActionConfig(namespace)
	if err != nil {
		return false, release.StatusUnknown, fmt.Errorf("failed to init helm action config: %w", err)
	}

	histClient := action.NewHistory(actionConfig)
	histClient.Max = 1
	versions, err := histClient.Run(name)

	if err == driver.ErrReleaseNotFound {
		return false, release.StatusUnknown, nil
	}

	if err != nil {
		return false, release.StatusUnknown, fmt.Errorf("failed to check helm release hustory: %w", err)
	}

	if len(versions) == 0 {
		return false, release.StatusUnknown, nil
	}

	lastVersion := versions[len(versions)-1]
	if lastVersion.Info.Status == release.StatusUninstalled {
		return false, lastVersion.Info.Status, nil
	}

	return true, lastVersion.Info.Status, nil
}

func (c *Client) install(ctx context.Context, chartPath string, releaseNamespace string, releaseName string, values map[string]any) (HelmResult, error) {
	log := logr.FromContextOrDiscard(ctx).WithName("helm-install").WithValues("chart", releaseName)
	log.Info("starting install")

	chart, err := loader.Load(chartPath)
	if err != nil {
		return HelmResult{}, fmt.Errorf("failed to load chart at %s: %w", chartPath, err)
	}

	actionConfig, err := newHelmActionConfig(releaseNamespace)
	if err != nil {
		return HelmResult{}, fmt.Errorf("failed to init helm action config: %w", err)
	}

	installAction := action.NewInstall(actionConfig)
	installAction.Namespace = releaseNamespace
	installAction.ReleaseName = releaseName

	rel, err := installAction.Run(chart, values)
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

func (c *Client) upgrade(ctx context.Context, chartPath string, releaseNamespace string, releaseName string, values map[string]any) (HelmResult, error) {
	log := logr.FromContextOrDiscard(ctx).WithName("helm-upgrade").WithValues("chart", releaseName)
	log.Info("starting upgrade")

	chart, err := loader.Load(chartPath)
	if err != nil {
		return HelmResult{}, fmt.Errorf("failed to load chart at %s: %w", chartPath, err)
	}

	actionConfig, err := newHelmActionConfig(releaseNamespace)
	if err != nil {
		return HelmResult{}, fmt.Errorf("failed to init helm action config: %w", err)
	}

	upgradeAction := action.NewUpgrade(actionConfig)
	upgradeAction.Namespace = releaseNamespace
	upgradeAction.Install = true

	rel, err := upgradeAction.Run(releaseName, chart, values)
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
