package installable

import (
	"context"
	"fmt"

	"github.com/kyma-project/cfapi/api/v1alpha1"
)

type Predicate func(ctx context.Context, config v1alpha1.InstallationConfig) bool

type Conditional struct {
	predicate Predicate
	delegate  Installable
}

func NewConditional(predicate Predicate, delegate Installable) *Conditional {
	return &Conditional{
		predicate: predicate,
		delegate:  delegate,
	}
}

func (c *Conditional) Name() string {
	return fmt.Sprintf("Conditional Installable: %s", c.delegate.Name())
}

func (c *Conditional) Install(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder EventRecorder) (Result, error) {
	if !c.predicate(ctx, config) {
		return Result{
			State:   ResultStateSuccess,
			Message: "Skipped installation",
		}, nil
	}

	return c.delegate.Install(ctx, config, eventRecorder)
}

func (c *Conditional) Uninstall(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder EventRecorder) (Result, error) {
	if !c.predicate(ctx, config) {
		return Result{
			State:   ResultStateSuccess,
			Message: "Skipped uninstallation",
		}, nil
	}

	return c.delegate.Uninstall(ctx, config, eventRecorder)
}
