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

// ResourceInfo represents the resource requests, limits, and usage for a container within a workload.
type ResourceInfo struct {
	Release    string `json:"release"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Replicas   string `json:"replicas,omitempty"`
	Container  string `json:"container"`
	CPURequest int64  `json:"cpu_request,omitempty"`    // millicores
	CPULimit   int64  `json:"cpu_limit,omitempty"`      // millicores
	MemRequest int64  `json:"memory_request,omitempty"` // bytes
	MemLimit   int64  `json:"memory_limit,omitempty"`   // bytes
	CPUUsage   int64  `json:"cpu_usage,omitempty"`      // millicores
	MemUsage   int64  `json:"memory_usage,omitempty"`   // bytes
}

// ResourceRecommendation represents resource recommendation for a container within a workload.
type ResourceRecommendation struct {
	Release               string
	Kind                  string
	Name                  string
	Container             string
	CPUUsage              int64 // millicores
	MemUsage              int64 // bytes
	CurrentCPURequest     int64 // millicores
	RecommendedCPURequest int64 // millicores
	CurrentMemRequest     int64 // bytes
	RecommendedMemRequest int64 // bytes
	CurrentCPULimit       int64 // millicores
	RecommendedCPULimit   int64 // millicores
	CurrentMemLimit       int64 // bytes
	RecommendedMemLimit   int64 // bytes
}
