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
	"context"
	"fmt"

	v1prometheus "github.com/prometheus/client_golang/api/prometheus/v1"

	"github.com/sergelogvinov/helm-resources/pkg/metrics"
	"github.com/sergelogvinov/helm-resources/pkg/resources"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	vpa "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/yaml"
)

func extractResourcesFromCRD(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	vpaClient vpa.Interface,
	prometheusClient v1prometheus.API,
	release string,
	manifest string,
	namespace,
	metricsWindow,
	aggregation string,
) ([]resources.ResourceInfo, error) {
	var res []resources.ResourceInfo

	var obj unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(manifest), &obj); err != nil {
		return nil, err // Skip invalid YAML
	}

	kind := obj.GetKind()
	apiVersion := obj.GetAPIVersion()

	switch apiVersion + "/" + kind { //nolint:gocritic
	case "postgresql.cnpg.io/v1/Cluster":
		return extractCNPGClusterResources(ctx, clientset, vpaClient, prometheusClient, release, obj, namespace, metricsWindow, aggregation)
	case "postgresql.cnpg.io/v1/Pooler":
		return extractCNPGPoolerResources(ctx, clientset, vpaClient, prometheusClient, release, obj, namespace, metricsWindow, aggregation)
	}

	return res, nil
}

// extractCNPGClusterResources extracts resource information from a CNPG Cluster resource
func extractCNPGClusterResources(
	ctx context.Context,
	_ *kubernetes.Clientset,
	vpaClient vpa.Interface,
	prometheusClient v1prometheus.API,
	release string,
	obj unstructured.Unstructured,
	namespace,
	metricsWindow,
	aggregation string,
) ([]resources.ResourceInfo, error) {
	clusterName := obj.GetName()

	replicas := "unknown"
	if instances, ok, err := unstructured.NestedInt64(obj.Object, "spec", "instances"); err == nil && ok {
		replicas = fmt.Sprintf("%d", instances)
	}

	resInfo := resources.ResourceInfo{
		Release:   release,
		Kind:      "Cluster",
		Name:      clusterName,
		Replicas:  replicas,
		Container: "postgres",
	}

	resourcesSpec, ok, err := unstructured.NestedMap(obj.Object, "spec", "resources")
	if err != nil || !ok {
		return []resources.ResourceInfo{resInfo}, err
	}

	if len(resourcesSpec) > 0 {
		extractContainerResources(resourcesSpec, &resInfo)
	}

	cpuUsage, memUsage := metrics.GetContainerMetrics(ctx, vpaClient, prometheusClient, namespace, resInfo.Kind, clusterName, resInfo.Container, metricsWindow, aggregation)
	resInfo.CPUUsage = cpuUsage
	resInfo.MemUsage = memUsage

	return []resources.ResourceInfo{resInfo}, nil
}

// extractCNPGPoolerResources extracts resource information from a CNPG Pooler resource
func extractCNPGPoolerResources(
	ctx context.Context,
	_ *kubernetes.Clientset,
	vpaClient vpa.Interface,
	prometheusClient v1prometheus.API,
	release string,
	obj unstructured.Unstructured,
	namespace,
	metricsWindow,
	aggregation string,
) ([]resources.ResourceInfo, error) {
	poolerName := obj.GetName()

	replicas := "unknown"
	if instances, ok, err := unstructured.NestedInt64(obj.Object, "spec", "instances"); err == nil && ok {
		replicas = fmt.Sprintf("%d", instances)
	}

	resInfo := resources.ResourceInfo{
		Release:   release,
		Kind:      "Pooler",
		Name:      poolerName,
		Replicas:  replicas,
		Container: "pgbouncer",
	}

	containers, ok, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
	if err != nil || !ok {
		return []resources.ResourceInfo{resInfo}, err
	}

	if len(containers) > 0 {
		// Use the first container (pgbouncer)
		if containerMap, ok := containers[0].(map[string]any); ok {
			resourcesSpec, ok, err := unstructured.NestedMap(containerMap, "resources")
			if err != nil || !ok {
				return []resources.ResourceInfo{resInfo}, err
			}

			extractContainerResources(resourcesSpec, &resInfo)
		}
	}

	cpuUsage, memUsage := metrics.GetContainerMetrics(ctx, vpaClient, prometheusClient, namespace, resInfo.Kind, poolerName, resInfo.Container, metricsWindow, aggregation)
	resInfo.CPUUsage = cpuUsage
	resInfo.MemUsage = memUsage

	return []resources.ResourceInfo{resInfo}, nil
}
