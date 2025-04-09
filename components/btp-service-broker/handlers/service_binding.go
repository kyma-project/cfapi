package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	btpv1 "github.com/SAP/sap-btp-service-operator/api/v1"
	"github.com/kyma-project/cfapi/osbapi"
	"github.com/kyma-project/cfapi/routing"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	ServiceBindingPath              = "/v2/service_instances/{instance_id}/service_bindings/{binding_id}"
	ServiceBindingLastOperationPath = "/v2/service_instances/{instance_id}/service_bindings/{binding_id}/last_operation"
)

type ServiceBindings struct {
	k8sClient     client.WithWatch
	rootNamespace string
}

func NewServiceBindings(k8sClient client.WithWatch, rootNamespace string) *ServiceBindings {
	return &ServiceBindings{k8sClient: k8sClient, rootNamespace: rootNamespace}
}

func (h *ServiceBindings) bind(r *http.Request) (*routing.Response, error) {
	if r.FormValue("accepts_incomplete") != "true" {
		return nil, osbapi.NewAsyncRequiredError("service binding requires client support for asynchronous service operations")
	}

	bindRequest := &osbapi.BindRequest{}
	err := decodeJSONPayload(r, bindRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to decode request body: %w", err)
	}

	paramsBytes, err := json.Marshal(bindRequest.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %w", err)
	}

	bindingId := routing.URLParam(r, "binding_id")
	instanceId := routing.URLParam(r, "instance_id")
	btpServiceBinding := &btpv1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.rootNamespace,
			Name:      bindingId,
		},
	}

	_, err = controllerutil.CreateOrPatch(r.Context(), h.k8sClient, btpServiceBinding, func() error {
		btpServiceBinding.Spec = btpv1.ServiceBindingSpec{
			ServiceInstanceName: instanceId,
			SecretName:          bindingId,
			Parameters:          &runtime.RawExtension{Raw: paramsBytes},
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create btp service binding: %w", err)
	}

	return routing.NewResponse(http.StatusAccepted), nil
}

func (h *ServiceBindings) get(r *http.Request) (*routing.Response, error) {
	bindingId := routing.URLParam(r, "binding_id")
	btpServiceBinding := &btpv1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.rootNamespace,
			Name:      bindingId,
		},
	}

	err := h.k8sClient.Get(r.Context(), client.ObjectKeyFromObject(btpServiceBinding), btpServiceBinding)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, osbapi.NewNotFoundError(err, fmt.Sprintf("service binding %q not found", bindingId))
		}
		return nil, err
	}

	bindingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.rootNamespace,
			Name:      btpServiceBinding.Name,
		},
	}

	err = h.k8sClient.Get(r.Context(), client.ObjectKeyFromObject(bindingSecret), bindingSecret)
	if err != nil {
		return nil, err
	}

	credentials, err := mapFromSecret(bindingSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to process binding credentials secret: %w", err)
	}

	paramsMap := map[string]any{}
	if btpServiceBinding.Spec.Parameters != nil {
		err = json.Unmarshal(btpServiceBinding.Spec.Parameters.Raw, &paramsMap)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal service instance parameters: %w", err)
		}
	}

	return routing.NewResponse(http.StatusOK).WithBody(osbapi.ServiceBindingResponse{
		Credentials: credentials,
		Parameters:  paramsMap,
	}), nil
}

func (h *ServiceBindings) delete(r *http.Request) (*routing.Response, error) {
	bindingId := routing.URLParam(r, "binding_id")
	btpServiceBinding := &btpv1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.rootNamespace,
			Name:      bindingId,
		},
	}

	err := h.k8sClient.Delete(r.Context(), btpServiceBinding)
	if client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	return routing.NewResponse(http.StatusAccepted), nil
}

func (h *ServiceBindings) lastOperation(r *http.Request) (*routing.Response, error) {
	bindingId := routing.URLParam(r, "binding_id")
	btpServiceBinding := &btpv1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.rootNamespace,
			Name:      bindingId,
		},
	}

	err := h.k8sClient.Get(r.Context(), client.ObjectKeyFromObject(btpServiceBinding), btpServiceBinding)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, osbapi.NewNotFoundError(err, fmt.Sprintf("service binding %q not found", bindingId))
		}
		return nil, err
	}

	return routing.NewResponse(http.StatusOK).WithBody(bindingStateResponse(btpServiceBinding)), nil
}

func bindingStateResponse(btpServiceBinding *btpv1.ServiceBinding) map[string]string {
	succeededCondition := meta.FindStatusCondition(btpServiceBinding.Status.Conditions, "Succeeded")
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

func (h *ServiceBindings) Routes() []routing.Route {
	return []routing.Route{
		{Method: "PUT", Pattern: ServiceBindingPath, Handler: h.bind},
		{Method: "GET", Pattern: ServiceBindingPath, Handler: h.get},
		{Method: "DELETE", Pattern: ServiceBindingPath, Handler: h.delete},
		{Method: "GET", Pattern: ServiceBindingLastOperationPath, Handler: h.lastOperation},
	}
}

func mapFromSecret(secret *corev1.Secret) (map[string]any, error) {
	convertedMap := make(map[string]any)
	for k := range secret.Data {
		if k == ".metadata" {
			continue
		}
		var err error
		convertedMap[k], err = parseValue(secret, k)
		if err != nil {
			return nil, err
		}
	}

	return convertedMap, nil
}

type propertyMetadata struct {
	Name   string `json:"name"`
	Format string `json:"format"`
}

func parseValue(bindingSecret *corev1.Secret, key string) (any, error) {
	valueFormat, err := getValueFormat(bindingSecret, key)
	if err != nil {
		return nil, err
	}

	switch valueFormat {
	case "text":
		return string(bindingSecret.Data[key]), nil
	case "json":
		var value any
		err := json.Unmarshal(bindingSecret.Data[key], &value)
		if err != nil {
			return nil, err
		}
		return value, nil

	}

	return nil, fmt.Errorf("unsupported value format %q for key %q in secret %s/%s", valueFormat, key, bindingSecret.Namespace, bindingSecret.Name)
}

func getValueFormat(bindingSecret *corev1.Secret, key string) (string, error) {
	secretMetadata, ok := bindingSecret.Data[".metadata"]
	if !ok {
		return "text", nil
	}

	var metadata map[string][]propertyMetadata
	if err := json.Unmarshal(secretMetadata, &metadata); err != nil {
		return "", fmt.Errorf("failed to unmarshal metadata from secret %s/%s: %w", bindingSecret.Namespace, bindingSecret.Name, err)
	}

	for _, properties := range metadata {
		if valueFormat := getPropertyFormat(properties, key); valueFormat != "" {
			return valueFormat, nil
		}
	}

	return "text", nil
}

func getPropertyFormat(credentialProperties []propertyMetadata, propertyName string) string {
	for _, property := range credentialProperties {
		if property.Name == propertyName {
			return property.Format
		}
	}
	return ""
}
