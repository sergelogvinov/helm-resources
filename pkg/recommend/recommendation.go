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
			Chart:     r.Chart,
			Release:   r.Release,
			Kind:      r.Kind,
			Name:      r.Name,
			Container: r.Container,
		}

		needsUpdate := false

		rec.CPUUsage = r.CPUUsage
		rec.MemUsage = r.MemUsage
		rec.CurrentCPURequest = r.CPURequest
		rec.CurrentMemRequest = r.MemRequest
		rec.CurrentCPULimit = r.CPULimit
		rec.CurrentMemLimit = r.MemLimit

		if r.CPUUsage > 0 && r.CPURequest > 0 && r.CPUUsage > r.CPURequest {
			recommendedCPU := roundUpCPULow(r.CPUUsage)
			rec.RecommendedCPURequest = recommendedCPU

			recommendedCPULimit := roundUpCPUHigh(r.CPUUsage)
			if recommendedCPULimit >= rec.CurrentCPULimit {
				rec.RecommendedCPULimit = recommendedCPULimit
			}

			needsUpdate = true
		}

		if r.MemUsage > 0 && r.MemRequest > 0 && r.MemUsage > r.MemRequest {
			recommendedMem := roundUpMemoryLow(r.MemUsage)
			rec.RecommendedMemRequest = recommendedMem

			recommendedMemLimit := roundUpMemoryHigh(r.MemUsage)
			if recommendedMemLimit >= rec.CurrentMemLimit {
				rec.RecommendedMemLimit = recommendedMemLimit
			}

			needsUpdate = true
		}

		if needsUpdate {
			recommendations = append(recommendations, rec)
		}
	}

	return recommendations
}
