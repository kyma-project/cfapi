// Package btp
package btp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"code.cloudfoundry.org/brokerapi/v13/domain"
	"code.cloudfoundry.org/brokerapi/v13/domain/apiresponses"
	"github.com/BooleanCat/go-functional/v2/it"
	btpv1 "github.com/SAP/sap-btp-service-operator/api/v1"
	"github.com/kyma-project/cfapi/components/btp-service-broker/tools"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/SAP/sap-btp-service-operator/client/sm"
	"github.com/SAP/sap-btp-service-operator/client/sm/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate -o fake -fake-name SMClient github.com/SAP/sap-btp-service-operator/client/sm.Client

type BTPBroker struct {
	k8sClient         client.Client
	smClient          sm.Client
	resourceNamespace string
}

func NewBroker(
	k8sClient client.Client,
	smClient sm.Client,
	resourceNamespace string,
) *BTPBroker {
	return &BTPBroker{
		k8sClient:         k8sClient,
		resourceNamespace: resourceNamespace,
		smClient:          smClient,
	}
}

func (b *BTPBroker) Services(ctx context.Context) ([]domain.Service, error) {
	offerings, err := b.smClient.ListOfferings(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get service offerings: %w", err)
	}

	plans, err := b.smClient.ListPlans(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get service plans: %w", err)
	}

	return it.TryCollect(it.MapError(slices.Values(offerings.ServiceOfferings), toService(plans.ServicePlans)))
}

func toService(plans []types.ServicePlan) func(types.ServiceOffering) (domain.Service, error) {
	return func(offering types.ServiceOffering) (domain.Service, error) {
		tags, err := unmarshal[[]string](offering.Tags)
		if err != nil {
			return domain.Service{}, fmt.Errorf("failed to unmarshal service offering tags: %w", err)
		}

		metadata, err := unmarshal[domain.ServiceMetadata](offering.Metadata)
		if err != nil {
			return domain.Service{}, fmt.Errorf("failed to unmarshal service offering metadata: %w", err)
		}

		offeringPlans, err := selectPlansForOffering(offering.ID, plans)
		if err != nil {
			return domain.Service{}, fmt.Errorf("failed to select plans for offering %s: %w", offering.ID, err)
		}

		requiredPermissions, err := unmarshal[[]domain.RequiredPermission](offering.Requires)
		if err != nil {
			return domain.Service{}, fmt.Errorf("failed to unmarshal required permissions: %w", err)
		}

		return domain.Service{
			ID:            offering.ID,
			Name:          offering.Name,
			Description:   offering.Description,
			Bindable:      offering.Bindable,
			Tags:          tools.ZeroIfNil(tags),
			PlanUpdatable: offering.PlanUpdatable,
			Plans:         offeringPlans,
			Requires:      tools.ZeroIfNil(requiredPermissions),
			Metadata:      metadata,
		}, nil
	}
}

func selectPlansForOffering(offeringID string, plans []types.ServicePlan) ([]domain.ServicePlan, error) {
	plansForOffering := it.Filter(slices.Values(plans), func(plan types.ServicePlan) bool {
		return plan.ServiceOfferingID == offeringID
	})

	return it.TryCollect(it.MapError(plansForOffering, toPlan))
}

func toPlan(plan types.ServicePlan) (domain.ServicePlan, error) {
	metadata, err := unmarshal[domain.ServicePlanMetadata](plan.Metadata)
	if err != nil {
		return domain.ServicePlan{}, fmt.Errorf("failed to unmarshal service plan metadata: %w", err)
	}

	return domain.ServicePlan{
		ID:          plan.ID,
		Name:        plan.Name,
		Description: plan.Description,
		Free:        &plan.Free,
		Bindable:    &plan.Bindable,
		Metadata:    metadata,
	}, nil
}

func unmarshal[T any](data json.RawMessage) (*T, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var obj T
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}
	return &obj, nil
}

func (b *BTPBroker) Provision(ctx context.Context, instanceID string, details domain.ProvisionDetails, _ bool) (domain.ProvisionedServiceSpec, error) {
	plan, err := b.getPlanByID(details.PlanID)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, err
	}

	offering, err := b.getOfferingByID(details.ServiceID)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, err
	}

	btpServiceInstance := &btpv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: b.resourceNamespace,
			Name:      instanceID,
		},
	}

	_, err = controllerutil.CreateOrPatch(ctx, b.k8sClient, btpServiceInstance, func() error {
		btpServiceInstance.Spec = btpv1.ServiceInstanceSpec{
			ServiceOfferingName: offering.Name,
			ServicePlanName:     plan.Name,
			ServicePlanID:       plan.ID,
			Parameters:          &runtime.RawExtension{Raw: details.RawParameters},
		}
		return nil
	})
	if err != nil {
		return domain.ProvisionedServiceSpec{}, fmt.Errorf("failed to create btp service instance: %w", err)
	}

	return domain.ProvisionedServiceSpec{IsAsync: true, OperationData: "provision-" + instanceID}, nil
}

func (b *BTPBroker) getPlanByID(planID string) (*types.ServicePlan, error) {
	plans, err := b.smClient.ListPlans(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get service plans: %w", err)
	}
	for _, plan := range plans.ServicePlans {
		if plan.ID == planID {
			return &plan, nil
		}
	}
	return nil, fmt.Errorf("plan with ID %s not found", planID)
}

func (b *BTPBroker) getOfferingByID(offeringID string) (*types.ServiceOffering, error) {
	plans, err := b.smClient.ListOfferings(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get service offerings: %w", err)
	}
	for _, offering := range plans.ServiceOfferings {
		if offering.ID == offeringID {
			return &offering, nil
		}
	}
	return nil, fmt.Errorf("offering with ID %s not found", offeringID)
}

func (b *BTPBroker) Deprovision(ctx context.Context, instanceID string, details domain.DeprovisionDetails, _ bool) (domain.DeprovisionServiceSpec, error) {
	btsServiceInstance := &btpv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceID,
			Namespace: b.resourceNamespace,
		},
	}

	err := client.IgnoreNotFound(b.k8sClient.Delete(ctx, btsServiceInstance))
	if err != nil {
		return domain.DeprovisionServiceSpec{}, fmt.Errorf("failed to delete btp service instance: %w", err)
	}

	return domain.DeprovisionServiceSpec{IsAsync: true, OperationData: "deprovision-" + instanceID}, nil
}

func (b *BTPBroker) LastOperation(ctx context.Context, instanceID string, details domain.PollDetails) (domain.LastOperation, error) {
	if strings.HasPrefix(details.OperationData, "provision-") {
		return b.provisionLastOperation(ctx, instanceID)
	}

	if strings.HasPrefix(details.OperationData, "deprovision-") {
		return b.deprovisionLastOperation(ctx, instanceID)
	}

	return domain.LastOperation{}, fmt.Errorf("unknown operation %s", details.OperationData)
}

func (b *BTPBroker) provisionLastOperation(ctx context.Context, instanceID string) (domain.LastOperation, error) {
	btpServiceInstance := &btpv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: b.resourceNamespace,
			Name:      instanceID,
		},
	}

	err := b.k8sClient.Get(ctx, client.ObjectKeyFromObject(btpServiceInstance), btpServiceInstance)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return domain.LastOperation{}, apiresponses.ErrInstanceNotFound
		}
		return domain.LastOperation{}, err
	}

	return provisionInstanceStateResponse(btpServiceInstance), nil
}

func provisionInstanceStateResponse(btpServiceInstance *btpv1.ServiceInstance) domain.LastOperation {
	if meta.IsStatusConditionTrue(latestInstanceStatusConditions(btpServiceInstance), "Succeeded") {
		return domain.LastOperation{
			State: "succeeded",
		}
	}

	if meta.IsStatusConditionTrue(latestInstanceStatusConditions(btpServiceInstance), "Failed") {
		return domain.LastOperation{
			State:       domain.Failed,
			Description: meta.FindStatusCondition(btpServiceInstance.Status.Conditions, "Failed").Message,
		}
	}

	if btpServiceInstance.Status.OperationType == "create" {
		return domain.LastOperation{
			State: "in progress",
		}
	}

	return domain.LastOperation{}
}

func (b *BTPBroker) deprovisionLastOperation(ctx context.Context, instanceID string) (domain.LastOperation, error) {
	btpServiceInstance := &btpv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: b.resourceNamespace,
			Name:      instanceID,
		},
	}

	err := b.k8sClient.Get(ctx, client.ObjectKeyFromObject(btpServiceInstance), btpServiceInstance)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return domain.LastOperation{State: domain.Succeeded}, nil
		}
		return domain.LastOperation{}, err
	}

	return deprovisionInstanceStateResponse(btpServiceInstance), nil
}

func deprovisionInstanceStateResponse(btpServiceInstance *btpv1.ServiceInstance) domain.LastOperation {
	if meta.IsStatusConditionTrue(latestInstanceStatusConditions(btpServiceInstance), "Failed") {
		return domain.LastOperation{
			State:       domain.Failed,
			Description: meta.FindStatusCondition(btpServiceInstance.Status.Conditions, "Failed").Message,
		}
	}

	if btpServiceInstance.Status.OperationType == "delete" {
		return domain.LastOperation{State: domain.InProgress}
	}

	return domain.LastOperation{}
}

func (b *BTPBroker) Bind(ctx context.Context, instanceID, bindingID string, details domain.BindDetails, _ bool) (domain.Binding, error) {
	btpBinding := &btpv1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: b.resourceNamespace,
			Name:      bindingID,
		},
	}

	_, err := controllerutil.CreateOrPatch(ctx, b.k8sClient, btpBinding, func() error {
		btpBinding.Spec = btpv1.ServiceBindingSpec{
			ServiceInstanceName: instanceID,
			SecretName:          bindingID,
			Parameters:          &runtime.RawExtension{Raw: details.RawParameters},
		}
		return nil
	})
	if err != nil {
		return domain.Binding{}, fmt.Errorf("failed to create btp service binding: %w", err)
	}

	lastOperation, err := b.LastBindingOperation(ctx, instanceID, bindingID, domain.PollDetails{OperationData: "bind-" + bindingID})
	if err != nil {
		return domain.Binding{}, fmt.Errorf("failed to get last binding operation: %w", err)
	}

	if lastOperation.State == domain.Succeeded {
		credentials, err := b.getCredentials(ctx, btpBinding.Name)
		if err != nil {
			return domain.Binding{}, fmt.Errorf("failed to get binding credentials: %w", err)
		}

		return domain.Binding{Credentials: credentials, IsAsync: false, OperationData: "bind-" + bindingID}, nil
	}

	return domain.Binding{IsAsync: true, OperationData: "bind-" + bindingID}, nil
}

func (b *BTPBroker) getCredentials(ctx context.Context, instanceID string) (map[string][]byte, error) {
	bindingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: b.resourceNamespace,
			Name:      instanceID,
		},
	}

	err := b.k8sClient.Get(ctx, client.ObjectKeyFromObject(bindingSecret), bindingSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to get binding Secret: %w", err)
	}

	return bindingSecret.Data, nil
}

func (b *BTPBroker) Unbind(ctx context.Context, instanceID, bindingID string, details domain.UnbindDetails, _ bool) (domain.UnbindSpec, error) {
	btsServiceBinding := &btpv1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingID,
			Namespace: b.resourceNamespace,
		},
	}

	err := client.IgnoreNotFound(b.k8sClient.Delete(ctx, btsServiceBinding))
	if err != nil {
		return domain.UnbindSpec{}, fmt.Errorf("failed to delete btp service binding: %w", err)
	}

	return domain.UnbindSpec{IsAsync: true, OperationData: "unbind-" + bindingID}, nil
}

func (b *BTPBroker) LastBindingOperation(ctx context.Context, instanceID, bindingID string, details domain.PollDetails) (domain.LastOperation, error) {
	if strings.HasPrefix(details.OperationData, "bind-") {
		return b.bindLastOperation(ctx, bindingID)
	}

	if strings.HasPrefix(details.OperationData, "unbind-") {
		return b.unbindLastOperation(ctx, bindingID)
	}

	return domain.LastOperation{}, fmt.Errorf("unknown operation %s", details.OperationData)
}

func (b *BTPBroker) bindLastOperation(ctx context.Context, bindingID string) (domain.LastOperation, error) {
	btpBinding := &btpv1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: b.resourceNamespace,
			Name:      bindingID,
		},
	}

	err := b.k8sClient.Get(ctx, client.ObjectKeyFromObject(btpBinding), btpBinding)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return domain.LastOperation{}, apiresponses.ErrBindingDoesNotExist
		}
		return domain.LastOperation{}, err
	}

	return bindingStateResponse(btpBinding), nil
}

func bindingStateResponse(btpBinding *btpv1.ServiceBinding) domain.LastOperation {
	if meta.IsStatusConditionTrue(latestBindingStatusConditions(btpBinding), "Succeeded") {
		return domain.LastOperation{
			State: "succeeded",
		}
	}

	if meta.IsStatusConditionTrue(latestBindingStatusConditions(btpBinding), "Failed") {
		return domain.LastOperation{
			State:       domain.Failed,
			Description: meta.FindStatusCondition(btpBinding.Status.Conditions, "Failed").Message,
		}
	}

	if btpBinding.Status.OperationType == "create" {
		return domain.LastOperation{
			State: "in progress",
		}
	}

	return domain.LastOperation{}
}

func (b *BTPBroker) unbindLastOperation(ctx context.Context, bindingID string) (domain.LastOperation, error) {
	btpServiceBinding := &btpv1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: b.resourceNamespace,
			Name:      bindingID,
		},
	}

	err := b.k8sClient.Get(ctx, client.ObjectKeyFromObject(btpServiceBinding), btpServiceBinding)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return domain.LastOperation{State: domain.Succeeded}, nil
		}
		return domain.LastOperation{}, err
	}

	return unbindStateResponse(btpServiceBinding), nil
}

func unbindStateResponse(btpServiceBinding *btpv1.ServiceBinding) domain.LastOperation {
	if meta.IsStatusConditionTrue(latestBindingStatusConditions(btpServiceBinding), "Failed") {
		return domain.LastOperation{
			State:       domain.Failed,
			Description: meta.FindStatusCondition(btpServiceBinding.Status.Conditions, "Failed").Message,
		}
	}

	if btpServiceBinding.Status.OperationType == "delete" {
		return domain.LastOperation{State: domain.InProgress}
	}

	return domain.LastOperation{}
}

func latestBindingStatusConditions(btpBinding *btpv1.ServiceBinding) []metav1.Condition {
	return slices.Collect(it.Filter(slices.Values(btpBinding.Status.Conditions), func(c metav1.Condition) bool {
		return c.ObservedGeneration == btpBinding.Generation
	}))
}

func latestInstanceStatusConditions(btpInstance *btpv1.ServiceInstance) []metav1.Condition {
	return slices.Collect(it.Filter(slices.Values(btpInstance.Status.Conditions), func(c metav1.Condition) bool {
		return c.ObservedGeneration == btpInstance.Generation
	}))
}

func (b *BTPBroker) GetBinding(ctx context.Context, instanceID, bindingID string, details domain.FetchBindingDetails) (domain.GetBindingSpec, error) {
	return domain.GetBindingSpec{}, errors.New("not implemented")
}

func (b *BTPBroker) GetInstance(ctx context.Context, instanceID string, details domain.FetchInstanceDetails) (domain.GetInstanceDetailsSpec, error) {
	return domain.GetInstanceDetailsSpec{}, errors.New("not implemented")
}

func (b *BTPBroker) Update(ctx context.Context, instanceID string, details domain.UpdateDetails, _ bool) (domain.UpdateServiceSpec, error) {
	return domain.UpdateServiceSpec{}, errors.New("not implemented")
}
