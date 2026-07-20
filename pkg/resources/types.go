/*
Copyright 2026 Serge Logvinov.

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

// Package resources represents the data structures used for resource information and recommendations.
package resources

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceInfo represents the resource requests, limits, and usage for a container within a workload.
type ResourceInfo struct {
	Chart     string `json:"chart"`
	Release   string `json:"release"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Replicas  string `json:"replicas,omitempty"`
	Container string `json:"container"`
	// Labels associated with the workload
	Labels map[string]string `json:"labels,omitempty"`
	// Usage
	CPUUsage int64 `json:"cpu_usage,omitempty"`    // millicores
	MemUsage int64 `json:"memory_usage,omitempty"` // bytes
	// Requests
	CPURequest int64 `json:"cpu_request,omitempty"`    // millicores
	MemRequest int64 `json:"memory_request,omitempty"` // bytes
	// Limits
	CPULimit int64 `json:"cpu_limit,omitempty"`    // millicores
	MemLimit int64 `json:"memory_limit,omitempty"` // bytes
}

// ResourceRecommendation represents resource recommendation for a container within a workload.
type ResourceRecommendation struct {
	Chart     string
	Release   string
	Kind      string
	Name      string
	Container string
	CPUUsage  int64 // millicores
	MemUsage  int64 // bytes
	// Requests
	CurrentCPURequest     int64 // millicores
	RecommendedCPURequest int64 // millicores
	CurrentMemRequest     int64 // bytes
	RecommendedMemRequest int64 // bytes
	// Limits
	CurrentCPULimit     int64 // millicores
	RecommendedCPULimit int64 // millicores
	CurrentMemLimit     int64 // bytes
	RecommendedMemLimit int64 // bytes
}

// FilterLabels filters the provided labels to include only common labels used for identifying workloads.
func FilterLabels(labels map[string]string) map[string]string {
	commonLabels := map[string]string{
		"app.kubernetes.io/name":      "",
		"app.kubernetes.io/instance":  "",
		"app.kubernetes.io/component": "",
	}

	filteredLabels := make(map[string]string, len(commonLabels))

	for key, value := range labels {
		if _, isCommon := commonLabels[key]; isCommon {
			filteredLabels[key] = value
		}
	}

	return filteredLabels
}

// ListOptions constructs a metav1.ListOptions object with the provided labels as a label selector.
func ListOptions(labels map[string]string) metav1.ListOptions {
	listOptions := metav1.ListOptions{}

	if len(labels) > 0 {
		labelSelectors := make([]string, 0, len(labels))

		for key, value := range labels {
			labelSelectors = append(labelSelectors, fmt.Sprintf("%s=%s", key, value))
		}

		listOptions.LabelSelector = strings.Join(labelSelectors, ",")
	}

	return listOptions
}
