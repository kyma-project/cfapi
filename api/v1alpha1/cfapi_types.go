/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package v1alpha1 contains API Schema definitions for the component v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=operator.kyma-project.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	CFAPIKind Kind = "CFAPI"
	Version   Kind = "v1alpha1"
)

type Kind string

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "operator.kyma-project.io", Version: "v1alpha1"}

	ConditionTypeInstallation = "Installation"
	ConditionReasonReady      = "Ready"
)

type CFAPIStatus struct {
	Status `json:",inline"`

	// Conditions contain a set of conditionals to determine the State of Status.
	// If all Conditions are met, State is expected to be in StateReady.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// URL contains the URL that should be used by the cf CLI in order
	// to consume the CF API.
	URL string `json:"url,omitempty"`
}

func (s *CFAPIStatus) WithState(state State) *CFAPIStatus {
	s.State = state
	return s
}

func (s *CFAPIStatus) WithURL(url string) *CFAPIStatus {
	s.URL = url
	return s
}

func (s *CFAPIStatus) WithInstallConditionStatus(status metav1.ConditionStatus, objGeneration int64) *CFAPIStatus {
	if s.Conditions == nil {
		s.Conditions = make([]metav1.Condition, 0, 1)
	}

	condition := meta.FindStatusCondition(s.Conditions, ConditionTypeInstallation)

	if condition == nil {
		condition = &metav1.Condition{
			Type:    ConditionTypeInstallation,
			Reason:  ConditionReasonReady,
			Message: "installation is ready and resources can be used",
		}
	}

	condition.Status = status
	condition.ObservedGeneration = objGeneration
	meta.SetStatusCondition(&s.Conditions, *condition)
	return s
}

type CFAPISpec struct {
	RootNamespace      string   `json:"rootNamespace,omitempty"`
	AppImagePullSecret string   `json:"appImagePullSecret,omitempty"`
	UAA                string   `json:"uaa,omitempty"`
	CFAdmins           []string `json:"cfadmins,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"
//+kubebuilder:printcolumn:name="URL",type=string,JSONPath=".status.url"

// CFAPI is the Schema for the samples API.
type CFAPI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CFAPISpec   `json:"spec,omitempty"`
	Status CFAPIStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CFAPIList contains a list of CFAPI.
type CFAPIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CFAPI `json:"items"`
}
