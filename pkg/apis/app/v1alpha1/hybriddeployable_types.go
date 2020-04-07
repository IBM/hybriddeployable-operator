// Copyright 2019 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	dplv1alpha1 "github.com/IBM/multicloud-operators-deployable/pkg/apis/app/v1alpha1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

var (
	HostingHybridDeployable    = SchemeGroupVersion.Group + "/hosting-hybriddeployable"
	ControlledBy               = SchemeGroupVersion.Group + "controlled-by"
	OutputOf                   = SchemeGroupVersion.Group + "output-of"
	DependencyFrom             = SchemeGroupVersion.Group + "dependency-from"
	HybridDeployableController = "hybriddeployable"
	DefaultDeployerType        = "kubernetes"
)

type HybridTemplate struct {
	DeployerType string                `json:"deployerType,omitempty"`
	Template     *runtime.RawExtension `json:"template"`
}

type HybridPlacement struct {
	Deployers      []corev1.ObjectReference `json:"deployers,omitempty"`
	DeployerLabels *metav1.LabelSelector    `json:"deployerLabels,omitempty"`
	PlacementRef   *corev1.ObjectReference  `json:"placementRef,omitempty"`
}

// HybridDeployableSpec defines the desired state of HybridDeployable
type HybridDeployableSpec struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	HybridTemplates []HybridTemplate         `json:"hybridtemplates,omitempty"`
	Placement       *HybridPlacement         `json:"placement,omitempty"`
	Dependencies    []corev1.ObjectReference `json:"dependencies,omitempty"`
}

type PerDeployerStatus struct {
	dplv1alpha1.ResourceUnitStatus `json:",inline"`
	Outputs                        []corev1.ObjectReference `json:"outputs,omitempty"`
}

// HybridDeployableStatus defines the observed state of HybridDeployable
type HybridDeployableStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	PerDeployerStatus map[string]PerDeployerStatus `json:"perDeployerStatus,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HybridDeployable is the Schema for the hybriddeployables API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=hybriddeployables,scope=Namespaced
// +kubebuilder:resource:path=hybriddeployables,shortName=hdpl
type HybridDeployable struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HybridDeployableSpec   `json:"spec,omitempty"`
	Status HybridDeployableStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HybridDeployableList contains a list of HybridDeployable
type HybridDeployableList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HybridDeployable `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HybridDeployable{}, &HybridDeployableList{})
}
