package controllers

import (
	golog "log"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
)

func releaseExists(namespace, name string) bool {
	settings := cli.New()
	actionConfig := new(action.Configuration)
	err := actionConfig.Init(settings.RESTClientGetter(), namespace,
		"secret", golog.Printf)

	if err != nil {
		return false
	}

	histClient := action.NewHistory(actionConfig)
	histClient.Max = 1
	versions, err := histClient.Run(name)

	return !(err == driver.ErrReleaseNotFound || isReleaseUninstalled(versions))
}

func installRelease(chart *chart.Chart, namespace, name string, values map[string]interface{}, logger logr.Logger) error {

	settings := cli.New()
	actionConfig := new(action.Configuration)
	err := actionConfig.Init(settings.RESTClientGetter(), namespace,
		"secret", golog.Printf)

	if err != nil {
		logger.Error(err, "error during init of helm action config")
		return err
	}

	installClient := action.NewInstall(actionConfig)

	installClient.ReleaseName = name
	installClient.Namespace = namespace
	installClient.Timeout = 5 * time.Minute
	installClient.Wait = true

	_, err = installClient.Run(chart, values)

	if err != nil {
		logger.Error(err, "error during install of korifi helm chart")
		return err
	}

	return nil
}

func updateRelease(chart *chart.Chart, namespace, name string, values map[string]interface{}, logger logr.Logger) error {

	settings := cli.New()
	actionConfig := new(action.Configuration)
	err := actionConfig.Init(settings.RESTClientGetter(), namespace,
		"secret", golog.Printf)

	if err != nil {
		logger.Error(err, "error during init of helm action config")
		return err
	}

	upgradeClient := action.NewUpgrade(actionConfig)

	upgradeClient.Namespace = namespace

	upgradeClient.Install = true
	upgradeClient.Wait = true
	upgradeClient.Timeout = 5 * time.Minute

	_, err = upgradeClient.Run(name, chart, values)

	if err != nil {
		logger.Error(err, "error during deployment of korifi helm chart")
		return err
	}

	return nil
}

func isReleaseUninstalled(versions []*release.Release) bool {
	return len(versions) > 0 && versions[len(versions)-1].Info.Status == release.StatusUninstalled
}

/*
This will update map m1 with the values of map m2 doing deep update.
The purpose is to prepare HELM values from different YML sources
*/
func DeepUpdate(m1, m2 map[string]any) {
	for k, vn := range m2 {
		vo, found := m1[k]
		updated := false
		if found && (vo != nil) {
			ko := reflect.TypeOf(vo).Kind()
			kn := reflect.TypeOf(vn).Kind()
			if ko == reflect.Map && kn == reflect.Map {
				DeepUpdate(vo.(map[string]any), vn.(map[string]any))
				updated = true
			}
		}
		if !updated {
			m1[k] = vn
		}
	}
}
