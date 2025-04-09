package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	btpv1 "github.com/SAP/sap-btp-service-operator/api/v1"
	"github.com/kyma-project/cfapi/osbapi"
	"github.com/kyma-project/cfapi/routing"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	ServiceInstancePath              = "/v2/service_instances/{id}"
	ServiceInstanceLastOperationPath = "/v2/service_instances/{id}/last_operation"
)

type ServiceIntance struct {
	k8sClient     client.WithWatch
	rootNamespace string
}

func NewServiceIntances(k8sClient client.WithWatch, rootNamespace string) *ServiceIntance {
	return &ServiceIntance{
		k8sClient:     k8sClient,
		rootNamespace: rootNamespace,
	}
}

func (h *ServiceIntance) provision(r *http.Request) (*routing.Response, error) {
	if r.FormValue("accepts_incomplete") != "true" {
		return nil, osbapi.NewAsyncRequiredError("service provisioning requires client support for asynchronous service operations")
	}

	provisionRequest := &osbapi.ProvisionRequest{}
	err := decodeJSONPayload(r, provisionRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to decode request body: %w", err)
	}

	paramsBytes, err := json.Marshal(provisionRequest.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %w", err)
	}
	

	id := routing.URLParam(r, "id")
	btpServiceInstance := &btpv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.rootNamespace,
			Name:      id,
		},
	}

	_, err = controllerutil.CreateOrPatch(r.Context(), h.k8sClient, btpServiceInstance, func() error {
		btpServiceInstance.Spec = btpv1.ServiceInstanceSpec{
			ServiceOfferingName: provisionRequest.ServiceId,
			ServicePlanName:     provisionRequest.PlanId,
			Parameters:          &runtime.RawExtension{Raw: paramsBytes},
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create btp service instance: %w", err)
	}

	return routing.NewResponse(http.StatusAccepted).WithBody(map[string]string{
		"operation": "provision-" + id,
	}), nil
}

func (h *ServiceIntance) deprovision(r *http.Request) (*routing.Response, error) {
	id := routing.URLParam(r, "id")
	btpServiceInstance := &btpv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.rootNamespace,
			Name:      id,
		},
	}

	err := client.IgnoreNotFound(h.k8sClient.Delete(r.Context(), btpServiceInstance))
	if err != nil {
		return nil, fmt.Errorf("failed to delete btp service instance: %w", err)
	}

	return routing.NewResponse(http.StatusAccepted).WithBody(map[string]string{
		"operation": "deprovision-" + id,
	}), nil
}

func (h *ServiceIntance) lastOperation(r *http.Request) (*routing.Response, error) {
	id := routing.URLParam(r, "id")
	btpServiceInstance := &btpv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.rootNamespace,
			Name:      id,
		},
	}

	err := h.k8sClient.Get(r.Context(), client.ObjectKeyFromObject(btpServiceInstance), btpServiceInstance)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, osbapi.NewNotFoundError(err, fmt.Sprintf("service instance %q not found", id))
		}
		return nil, err
	}

	return routing.NewResponse(http.StatusOK).WithBody(instanceStateResponse(btpServiceInstance)), nil
}

func instanceStateResponse(btpServiceInstance *btpv1.ServiceInstance) map[string]string {
	succeededCondition := meta.FindStatusCondition(btpServiceInstance.Status.Conditions, "Succeeded")
	if succeededCondition != nil {
		if succeededCondition.Status == metav1.ConditionTrue && succeededCondition.Reason == "Created" {
			return map[string]string{
				"state": "succeeded",
			}
		}

		if succeededCondition.Status == metav1.ConditionFalse &&
			strings.HasPrefix(succeededCondition.Message, "BrokerError:") {
			return map[string]string{
				"state":       "failed",
				"description": succeededCondition.Message,
			}
		}

		if succeededCondition.Reason == "CreateInProgress" || succeededCondition.Reason == "DeleteInProgress" {
			return map[string]string{
				"state": "in progress",
			}
		}

	}

	return map[string]string{
		"state": "in progress",
	}
}

func decodeJSONPayload(r *http.Request, object any) error {
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	decoder.DisallowUnknownFields()
	return decoder.Decode(object)
}

func (h *ServiceIntance) Routes() []routing.Route {
	return []routing.Route{
		{Method: "PUT", Pattern: ServiceInstancePath, Handler: h.provision},
		{Method: "DELETE", Pattern: ServiceInstancePath, Handler: h.deprovision},
		{Method: "GET", Pattern: ServiceInstanceLastOperationPath, Handler: h.lastOperation},
	}
}
