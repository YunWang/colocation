/*

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

package v1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
// Important: Run "make" to regenerate code after modifying this file

// OfflineSpec defines the desired state of Offline
type OfflineSpec struct {
	MinGang  int32                 `json:"minGang,omitempty"`
	Level    int32                 `json:"level,omitempty"`
	Selector *metav1.LabelSelector `json:"selector"`
	Tasks    []*v1.PodTemplateSpec `json:"tasks"`
	Queue    string 			   `json:"queue,omitempty"`
}

type OfflinePhase string

const (
	//running+succeeded>=minGang
	OfflineRunningPhase    = "Running"
	//total = 0
	OfflinePendingPhase    = "Pending"
	//be preempted or evicted, and running+succeeded < minGang
	OfflineUnknownPhase    = "Unknown"
	//succeeded
	OfflineSucceededPhase  = "Succeeded"
	//
	OfflineSchedulingPhase = "Scheduling"
	//Failed to schedule,running+succeeded<minGang
	OfflineFailedPhase = "Failed"
)


// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
// Important: Run "make" to regenerate code after modifying this file
// OfflineStatus defines the observed state of Offline
type OfflineStatus struct {
	Phase        OfflinePhase `json:"phase,omitempty"`
	PodPending   int32        `json:"pending,omitempty"`
	PodRunning   int32        `json:"running,omitempty"`
	PodSucceeded int32        `json:"succeeded,omitempty"`
	PodFailed    int32        `json:"failed,omitempty"`
	PodUnknown   int32        `json:"unknown,omitempty"`
}

// +kubebuilder:object:root=true

// Offline is the Schema for the offlines API
type Offline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OfflineSpec   `json:"spec,omitempty"`
	Status OfflineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OfflineList contains a list of Offline
type OfflineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Offline `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Offline{}, &OfflineList{})
}
