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
/*
 * SPDX-FileCopyrightText: 2024 Samir Zeort <samir.zeort@sap.com>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

// Package v1alpha1 contains API Schema definitions for the component v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=operator.kyma-project.io
package v1alpha1

import (
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

	ConditionTypeConfiguration = "Configuration"
	ConditionTypeInstallation  = "Installation"
)

type CFAPIStatus struct {
	Status `json:",inline"`

	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	//+kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	//+kubebuilder:validation:Optional
	InstallationConfig InstallationConfig `json:"installationConfig,omitempty"`

	// URL contains the URL that should be used by the cf CLI in order
	// to consume the CF API.
	//+kubebuilder:validation:Optional
	URL string `json:"url,omitempty"`
}

type InstallationConfig struct {
	//+kubebuilder:validation:Optional
	RootNamespace string `json:"rootNamespace"`
	//+kubebuilder:validation:Optional
	ContainerRegistrySecret string `json:"containerRegistrySecret"`
	//+kubebuilder:validation:Optional
	UAAURL string `json:"uaaUrl"`
	//+kubebuilder:validation:Optional
	CFAdmins []string `json:"cfAdmins"`
	//+kubebuilder:validation:Optional
	CFDomain string `json:"cfDomain"`
	//+kubebuilder:validation:Optional
	KorifiIngressHost string `json:"korifiIngressHost"`
}

type CFAPISpec struct {
	RootNamespace           string   `json:"rootNamespace,omitempty"`
	ContainerRegistrySecret string   `json:"containerRegistrySecret,omitempty"`
	UAA                     string   `json:"uaa,omitempty"`
	CFAdmins                []string `json:"cfadmins,omitempty"`
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

func (a *CFAPI) StatusConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

// +kubebuilder:object:root=true

// CFAPIList contains a list of CFAPI.
type CFAPIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CFAPI `json:"items"`
}
