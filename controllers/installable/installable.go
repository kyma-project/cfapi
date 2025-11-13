package installable

import (
	"context"

	"github.com/kyma-project/cfapi/api/v1alpha1"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate -o fake -fake-name Installable . Installable
type Installable interface {
	Install(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder EventRecorder) (Result, error)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate -o fake -fake-name Uninstallable . Uninstallable
type Uninstallable interface {
	Uninstall(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder EventRecorder) (Result, error)
}

type Result struct {
	State   ResultState
	Message string
}

type ResultState int

const (
	ResultStateSuccess ResultState = iota
	ResultStateInProgress
	ResultStateFailed
)
