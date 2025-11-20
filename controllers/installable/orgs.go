package installable

import (
	"context"
	"fmt"

	korifiv1alpha1 "code.cloudfoundry.org/korifi/controllers/api/v1alpha1"
	"github.com/kyma-project/cfapi/api/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Orgs struct {
	k8sClient client.Client
}

func NewOrgs(k8sClient client.Client) *Orgs {
	return &Orgs{
		k8sClient: k8sClient,
	}
}

func (o *Orgs) Name() string {
	return "Orgs Installable"
}

func (o *Orgs) Install(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder EventRecorder) (Result, error) {
	panic("not supported")
}

func (o *Orgs) Uninstall(ctx context.Context, config v1alpha1.InstallationConfig, eventRecorder EventRecorder) (Result, error) {
	if err := o.deleteAllOrgs(ctx, config.RootNamespace); err != nil {
		eventRecorder.Event(EventWarning, "InstallableFailed", fmt.Sprintf("Installable %s failed", o.Name()))
		return Result{}, fmt.Errorf("failed to delete orgs: %w", err)
	}

	orgsCount, err := o.remainingOrgsCount(ctx)
	if err != nil {
		eventRecorder.Event(EventWarning, "InstallableFailed", fmt.Sprintf("Installable %s failed", o.Name()))
		return Result{}, fmt.Errorf("failed to list remaining orgs: %w", err)
	}

	if orgsCount > 0 {
		return Result{
			State:   ResultStateInProgress,
			Message: fmt.Sprintf("%d orgs remaining", orgsCount),
		}, nil
	}

	return Result{
		State:   ResultStateSuccess,
		Message: "Orgs deleted successfully",
	}, nil
}

func (o *Orgs) deleteAllOrgs(ctx context.Context, rootNs string) error {
	return client.IgnoreNotFound(o.k8sClient.DeleteAllOf(ctx, &korifiv1alpha1.CFOrg{}, client.InNamespace(rootNs)))
}

func (o *Orgs) remainingOrgsCount(ctx context.Context) (int, error) {
	orgList := &korifiv1alpha1.CFOrgList{}
	err := o.k8sClient.List(ctx, orgList)
	if k8serrors.IsNotFound(err) {
		return 0, nil
	}

	return len(orgList.Items), err
}
