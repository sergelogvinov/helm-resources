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

package cmd

import (
	"fmt"

	"github.com/sergelogvinov/helm-resources/pkg/resources"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func extractContainerResources(resourcesSpec map[string]any, resInfo *resources.ResourceInfo) {
	if requests, found, err := unstructured.NestedMap(resourcesSpec, "requests"); err == nil && found {
		if cpuRequest, found := requests["cpu"]; found {
			cpuStr := fmt.Sprintf("%v", cpuRequest)
			if cpuStr != "" {
				if cpuQuantity, err := resource.ParseQuantity(cpuStr); err == nil {
					resInfo.CPURequest = cpuQuantity.MilliValue()
				}
			}
		}

		if memRequest, found := requests["memory"]; found {
			memStr := fmt.Sprintf("%v", memRequest)
			if memStr != "" {
				if memQuantity, err := resource.ParseQuantity(memStr); err == nil {
					resInfo.MemRequest = memQuantity.Value()
				}
			}
		}
	}

	if limits, found, err := unstructured.NestedMap(resourcesSpec, "limits"); err == nil && found {
		if cpuLimit, found := limits["cpu"]; found {
			cpuStr := fmt.Sprintf("%v", cpuLimit)
			if cpuStr != "" {
				if cpuQuantity, err := resource.ParseQuantity(cpuStr); err == nil {
					resInfo.CPULimit = cpuQuantity.MilliValue()
				}
			}
		}

		if memLimit, found := limits["memory"]; found {
			memStr := fmt.Sprintf("%v", memLimit)
			if memStr != "" {
				if memQuantity, err := resource.ParseQuantity(memStr); err == nil {
					resInfo.MemLimit = memQuantity.Value()
				}
			}
		}
	}
}
