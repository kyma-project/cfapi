package installable

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyma-project/cfapi/api/v1alpha1"
)

//counterfeiter:generate -o fake -fake-name Condition . Condition
type Condition interface {
	IsMet(ctx context.Context, config v1alpha1.InstallationConfig) (bool, string)
}

type Conditional struct {
	condition Condition
	delegate  Installable
}

func NewConditional(condition Condition, delegate Installable) *Conditional {
	return &Conditional{
		condition: condition,
		delegate:  delegate,
	}
}

func (c *Conditional) Install(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder EventRecorder) (Result, error) {
	log := logr.FromContextOrDiscard(ctx).WithName("conditional")
	conditionMet, message := c.condition.IsMet(ctx, config)
	if !conditionMet {
		log.Info(fmt.Sprintf("condition not met: %s", message))
		return Result{
			State:   ResultStateInProgress,
			Message: message,
		}, nil
	}

	return c.delegate.Install(ctx, config, eventRecorder)
}
