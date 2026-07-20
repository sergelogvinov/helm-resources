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

package crds

import (
	"context"
	"fmt"

	"github.com/sergelogvinov/helm-resources/pkg/metrics"
	"github.com/sergelogvinov/helm-resources/pkg/resources"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

// extractClickHouseInstallationResources extracts resource information from a ClickHouseInstallation resource
//
//nolint:gocyclo,cyclop
func extractClickHouseInstallationResources(
	ctx context.Context,
	_ *kubernetes.Clientset,
	metricsClient *metrics.Client,
	release string,
	obj unstructured.Unstructured,
	namespace string,
) ([]resources.ResourceInfo, error) {
	installationName := obj.GetName()
	clusterName := installationName

	replicas := unknown
	totalReplicas := int64(0)

	clusters, found, err := unstructured.NestedSlice(obj.Object, "spec", "configuration", "clusters")
	if err == nil && found {
		for _, cluster := range clusters {
			if clusterMap, ok := cluster.(map[string]any); ok {
				shardsCount, ok, err := unstructured.NestedInt64(clusterMap, "layout", "shardsCount")
				if err == nil && ok {
					replicasCount, ok, err := unstructured.NestedInt64(clusterMap, "layout", "replicasCount")
					if err == nil && ok {
						totalReplicas += shardsCount * replicasCount

						continue
					}
				}

				name, ok, err := unstructured.NestedString(clusterMap, "name")
				if err == nil && ok {
					clusterName = name
				}
			}
		}
	}

	if totalReplicas > 0 {
		replicas = fmt.Sprintf("%d", totalReplicas)
	}

	resInfo := resources.ResourceInfo{
		Release:   release,
		Kind:      "ClickHouseInstallation",
		Name:      installationName,
		Replicas:  replicas,
		Container: "clickhouse",
	}

	podTemplates, found, err := unstructured.NestedSlice(obj.Object, "spec", "templates", "podTemplates")
	if err == nil && found && len(podTemplates) > 0 {
		if podTemplate, ok := podTemplates[0].(map[string]any); ok {
			containers, found, err := unstructured.NestedSlice(podTemplate, "spec", "containers")
			if err == nil && found {
				for _, container := range containers {
					if containerMap, ok := container.(map[string]any); ok {
						if containerName, found, _ := unstructured.NestedString(containerMap, "name"); found && containerName == "clickhouse" { //nolint:errcheck
							resourcesSpec, ok, err := unstructured.NestedMap(containerMap, "resources")
							if err == nil && ok {
								extractContainerResources(resourcesSpec, &resInfo)
							}

							break
						}
					}
				}
			}
		}
	}

	res := resInfo
	res.Name = "chi-" + installationName + "-" + clusterName

	cpuUsage, memUsage := metricsClient.GetContainerMetrics(ctx, namespace, res)
	resInfo.CPUUsage = cpuUsage
	resInfo.MemUsage = memUsage

	return []resources.ResourceInfo{resInfo}, nil
}
