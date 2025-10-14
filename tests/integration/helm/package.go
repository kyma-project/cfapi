package helm

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate -o fake -fake-name Installable github.com/kyma-project/cfapi/controllers/installable.Installable
//counterfeiter:generate -o fake -fake-name EventRecorder github.com/kyma-project/cfapi/controllers/installable.EventRecorder
//counterfeiter:generate -o fake -fake-name HelmValuesProvider github.com/kyma-project/cfapi/controllers/installable.HelmValuesProvider
