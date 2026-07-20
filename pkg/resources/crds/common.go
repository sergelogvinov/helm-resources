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

	"github.com/sergelogvinov/helm-resources/pkg/metrics"
	"github.com/sergelogvinov/helm-resources/pkg/resources"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/yaml"
)

const unknown = "unknown"

// ExtractResourcesFromCRD extracts resource information from a Custom Resource Definition (CRD) manifest for supported workloads.
func ExtractResourcesFromCRD(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	metricsClient *metrics.Client,
	release string,
	manifest string,
	namespace string,
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
		return extractCNPGClusterResources(ctx, clientset, metricsClient, release, obj, namespace)
	case "postgresql.cnpg.io/v1/Pooler":
		return extractCNPGPoolerResources(ctx, clientset, metricsClient, release, obj, namespace)
	case "clickhouse.altinity.com/v1/ClickHouseInstallation":
		return extractClickHouseInstallationResources(ctx, clientset, metricsClient, release, obj, namespace)
	}

	return res, nil
}
