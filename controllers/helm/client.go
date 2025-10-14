package helm

import (
	"fmt"

	golog "log"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Apply(chartPath string, releaseNamespace string, releaseName string, values map[string]any) error {
	chart, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart at %s: %w", chartPath, err)
	}

	actionConfig, err := newHelmActionConfig(releaseNamespace)
	if err != nil {
		return fmt.Errorf("failed to init helm action config: %w", err)
	}

	upgradeAction := action.NewUpgrade(actionConfig)
	upgradeAction.Namespace = releaseNamespace
	upgradeAction.Install = true

	_, err = upgradeAction.Run(releaseName, chart, values)
	if err != nil {
		return fmt.Errorf("failed to update release %s: %w", releaseName, err)
	}

	return nil
}

func (c *Client) IsReady(releaseNamespace string, releaseName string) (bool, error) {
	relStatus, err := c.getStatus(releaseNamespace, releaseName)
	if err != nil {
		return false, err
	}

	return relStatus == release.StatusDeployed, nil
}

func (c *Client) IsFailed(releaseNamespace string, releaseName string) (bool, error) {
	relStatus, err := c.getStatus(releaseNamespace, releaseName)
	if err != nil {
		return false, err
	}

	return relStatus == release.StatusFailed, nil
}

func (c *Client) getStatus(releaseNamespace string, releaseName string) (release.Status, error) {
	helmRelease, err := c.getHelmRelease(releaseNamespace, releaseName)
	if err != nil {
		return release.StatusUnknown, err
	}

	return helmRelease.Info.Status, nil
}

func (c *Client) getHelmRelease(releaseNamespace string, releaseName string) (*release.Release, error) {
	actionConfig, err := newHelmActionConfig(releaseNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to init helm action config: %w", err)
	}
	getAction := action.NewGet(actionConfig)
	release, err := getAction.Run(releaseName)
	if err != nil {
		return nil, fmt.Errorf("failed to get release %s in namespace %s: %w", releaseName, releaseNamespace, err)
	}
	return release, nil
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
