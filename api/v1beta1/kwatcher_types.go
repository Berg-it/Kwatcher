/*
Copyright 2025.

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

package v1beta1

import (
	"fmt"
	"regexp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KwatcherSpec defines the desired state of Kwatcher.
type KwatcherSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Kwatcher. Edit kwatcher_types.go to remove/update
	Provider KwatcherProvider `json:"provider"`
	Config   KwatcherConfig   `json:"config"`
}

type KwatcherProvider struct {
	Url  string `json:"url"`  // Specifies the url of provider
	Port int32  `json:"port"` // Specifies the port of provider
}

type KwatcherConfig struct {
	RefreshInterval int32  `json:"refreshInterval"` // Specifies the refresh interval of kwatcher
	Secret          string `json:"secret"`          // Specifies the secret of provider
}

// KwatcherStatus defines the observed state of Kwatcher.
type KwatcherStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Kwatcher is the Schema for the kwatchers API.
type Kwatcher struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KwatcherSpec   `json:"spec,omitempty"`
	Status KwatcherStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KwatcherList contains a list of Kwatcher.
type KwatcherList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kwatcher `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Kwatcher{}, &KwatcherList{})
}

// Validate checks if all required fields are filled
func (s *KwatcherSpec) Validate() error {
	if s.Provider.Url == "" {
		return fmt.Errorf("provider URL cannot be empty")
	}

	// Validate URL format
	urlRegex := regexp.MustCompile(`^(http|https)://[a-zA-Z0-9\-\.]+\.[a-zA-Z]{2,}(:[0-9]+)?(/.*)?$`)
	if !urlRegex.MatchString(s.Provider.Url) {
		return fmt.Errorf("provider URL must be a valid HTTP/HTTPS URL")
	}

	if s.Provider.Port <= 0 {
		return fmt.Errorf("provider port must be positive")
	}
	if s.Config.RefreshInterval <= 0 {
		return fmt.Errorf("refresh interval must be positive")
	}

	return nil
}
