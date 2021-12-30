/*
Copyright 2021 nakamasato.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretMirrorSpec defines the desired state of SecretMirror
type SecretMirrorSpec struct {
	// FromNamespace is a namespace from which the target Secret is mirrored.
	// +kubebuilder:validation:Required
	FromNamespace string `json:"fromNamespace"`
}

// SecretMirrorStatus defines the observed state of SecretMirror
type SecretMirrorStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SecretMirror is the Schema for the secretmirrors API
type SecretMirror struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretMirrorSpec   `json:"spec,omitempty"`
	Status SecretMirrorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SecretMirrorList contains a list of SecretMirror
type SecretMirrorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretMirror `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecretMirror{}, &SecretMirrorList{})
}
