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

// Package recommend provides functionality to analyze resource usage and generate recommendations for Kubernetes workloads.
package recommend

import (
	"github.com/sergelogvinov/helm-resources/pkg/resources"
)

// AnalyzeRecommendations analyzes the provided resource information and generates recommendations
// for CPU and memory requests and limits based on observed usage.
func AnalyzeRecommendations(res []resources.ResourceInfo) []resources.ResourceRecommendation {
	var recommendations []resources.ResourceRecommendation

	for _, r := range res {
		if r.CPUUsage == 0 || r.MemUsage == 0 {
			continue
		}

		rec := resources.ResourceRecommendation{
			Kind:      r.Kind,
			Name:      r.Name,
			Container: r.Container,
		}

		needsUpdate := false

		rec.CPUUsage = r.CPUUsage
		rec.MemUsage = r.MemUsage
		rec.CurrentCPURequest = r.CPURequest
		rec.CurrentMemRequest = r.MemRequest
		rec.RecommendedCPURequest = r.CPURequest
		rec.RecommendedMemRequest = r.MemRequest
		rec.CurrentCPULimit = r.CPULimit
		rec.CurrentMemLimit = r.MemLimit
		rec.RecommendedCPULimit = r.CPULimit
		rec.RecommendedMemLimit = r.MemLimit

		if r.CPUUsage > 0 && r.CPURequest > 0 && r.CPUUsage > r.CPURequest {
			recommendedCPU := roundUpCPU(int64(float64(r.CPUUsage) * 1.2))
			rec.RecommendedCPURequest = recommendedCPU

			recommendedCPULimit := roundUpCPU(int64(float64(r.CPUUsage) * 2.0))
			rec.RecommendedCPULimit = recommendedCPULimit

			needsUpdate = true
		}

		if r.MemUsage > 0 && r.MemRequest > 0 && r.MemUsage > r.MemRequest {
			recommendedMem := roundUpMemory(int64(float64(r.MemUsage) * 1.2))
			rec.RecommendedMemRequest = recommendedMem

			recommendedMemLimit := roundUpMemory(int64(float64(r.MemUsage) * 2.0))
			rec.RecommendedMemLimit = recommendedMemLimit

			needsUpdate = true
		}

		if needsUpdate {
			recommendations = append(recommendations, rec)
		}
	}

	return recommendations
}

func roundUpCPU(milliCores int64) int64 {
	if milliCores <= 0 {
		return int64(100)
	}

	increment := int64(500) // 500m

	return ((milliCores + increment - 1) / increment) * increment
}

func roundUpMemory(bytes int64) int64 {
	if bytes <= 0 {
		return int64(64 * 1024 * 1024)
	}

	increment := int64(128 * 1024 * 1024) // 128Mi in bytes

	return ((bytes + increment - 1) / increment) * increment
}
