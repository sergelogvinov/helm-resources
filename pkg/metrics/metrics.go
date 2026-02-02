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

// Package metrics provides functionality to retrieve resource usage metrics for Kubernetes workloads.
package metrics

import (
	"context"
	"fmt"
	"time"

	v1prometheus "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// GetContainerMetrics retrieves CPU and memory usage for a container using either Prometheus or Kubernetes Metrics API
func GetContainerMetrics(
	ctx context.Context,
	prometheusClient v1prometheus.API,
	namespace,
	workloadName,
	containerName,
	metricsWindow,
	aggregation string,
) (int64, int64) {
	if prometheusClient != nil {
		return GetPrometheusMetrics(ctx, prometheusClient, namespace, workloadName, containerName, metricsWindow, aggregation)
	}

	return 0, 0
}

// GetPrometheusMetrics retrieves CPU and memory usage from Prometheus
func GetPrometheusMetrics(ctx context.Context, prometheusClient v1prometheus.API, namespace, workloadName, containerName, metricsWindow, aggregation string) (int64, int64) {
	if aggregation != "avg" && aggregation != "max" {
		aggregation = "avg" // default fallback
	}

	cpuQuery := fmt.Sprintf(`%s(rate(container_cpu_usage_seconds_total{namespace="%s",pod=~"%s.*",container="%s"}[%s])) * 1000`, aggregation, namespace, workloadName, containerName, metricsWindow)

	cpuResult, _, err := prometheusClient.Query(ctx, cpuQuery, time.Now())
	if err != nil {
		return 0, 0
	}

	memQuery := fmt.Sprintf(`%s(container_memory_usage_bytes{namespace="%s",pod=~"%s.*",container="%s"}[%s])`, aggregation, namespace, workloadName, containerName, metricsWindow)

	memResult, _, err := prometheusClient.Query(ctx, memQuery, time.Now())
	if err != nil {
		return 0, 0
	}

	var cpuUsage, memUsage int64

	if cpuVector, ok := cpuResult.(model.Vector); ok && len(cpuVector) > 0 {
		cpuUsage = int64(cpuVector[0].Value)
	}

	if memVector, ok := memResult.(model.Vector); ok && len(memVector) > 0 {
		memUsage = int64(memVector[0].Value)
	}

	return cpuUsage, memUsage
}
