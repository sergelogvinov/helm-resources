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
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	v1prometheus "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/sergelogvinov/helm-resources/pkg/resources"

	v1 "k8s.io/api/core/v1"
	vpa "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/client/clientset/versioned"
	"k8s.io/client-go/rest"
	metricsv1 "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Client provides methods to retrieve resource usage metrics for Kubernetes workloads.
type Client struct {
	vpaClient        vpa.Interface
	prometheusClient v1prometheus.API
	metricsClient    metricsv1.Interface
	metricsWindow    string
	aggregation      string
}

// New creates a new Client with the provided clients and configuration.
// All parameters are optional; pass nil for clients you don't want to use.
// metricsWindow specifies the time window for Prometheus queries (e.g., "5m", "1h").
// aggregation specifies the aggregation function for Prometheus queries ("avg" or "max").
func New(
	prometheusURL string,
	metricsWindow string,
	aggregation string,
	config *rest.Config,
) (*Client, error) {
	var (
		prometheusClient v1prometheus.API
		vpaClient        vpa.Interface
		metricsClient    metricsv1.Interface
	)

	if prometheusURL != "" {
		promClient, err := api.NewClient(api.Config{
			Address: prometheusURL,
			RoundTripper: &http.Transport{
				IdleConnTimeout: 30 * time.Second,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Prometheus client: %w", err)
		}

		prometheusClient = v1prometheus.NewAPI(promClient)
	}

	if config != nil {
		vpaClientset, err := vpa.NewForConfig(config)
		if err == nil {
			vpaClient = vpaClientset
		}
	}

	if config != nil {
		metricsClientset, err := metricsv1.NewForConfig(config)
		if err == nil {
			metricsClient = metricsClientset
		}
	}

	if aggregation != "avg" && aggregation != "max" {
		aggregation = "avg" // default fallback
	}

	return &Client{
		vpaClient:        vpaClient,
		prometheusClient: prometheusClient,
		metricsClient:    metricsClient,
		metricsWindow:    metricsWindow,
		aggregation:      aggregation,
	}, nil
}

// GetContainerMetrics retrieves CPU and memory usage for a container using the configured metrics source.
// Returns CPU in millicores and memory in bytes.
func (m *Client) GetContainerMetrics(
	ctx context.Context,
	namespace string,
	res resources.ResourceInfo,
) (int64, int64) {
	if m.prometheusClient != nil {
		return m.getPrometheusMetrics(ctx, namespace, res)
	}

	var (
		cpu int64
		mem int64
	)

	if m.vpaClient != nil && cpu == 0 && mem == 0 {
		cpu, mem = m.getVPAMetrics(ctx, namespace, res)
	}

	if m.metricsClient != nil {
		cpu, mem = m.getKubernetesMetrics(ctx, namespace, res)
	}

	return cpu, mem
}

// getPrometheusMetrics retrieves CPU and memory usage from Prometheus
func (m *Client) getPrometheusMetrics(ctx context.Context, namespace string, res resources.ResourceInfo) (int64, int64) {
	cpuQuery := fmt.Sprintf(`%s(rate(container_cpu_usage_seconds_total{namespace="%s",pod=~"%s.*",container="%s"}[%s])) * 1000`, m.aggregation, namespace, res.Name, res.Container, m.metricsWindow)

	cpuResult, _, err := m.prometheusClient.Query(ctx, cpuQuery, time.Now())
	if err != nil {
		return 0, 0
	}

	memQuery := fmt.Sprintf(`%s(container_memory_usage_bytes{namespace="%s",pod=~"%s.*",container="%s"}[%s])`, m.aggregation, namespace, res.Name, res.Container, m.metricsWindow)

	memResult, _, err := m.prometheusClient.Query(ctx, memQuery, time.Now())
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

// getKubernetesMetrics retrieves CPU and Memory usage for a container from the
// Kubernetes Metrics API (metrics.k8s.io/v1).
func (m *Client) getKubernetesMetrics(ctx context.Context, namespace string, res resources.ResourceInfo) (int64, int64) {
	podMetricsList, err := m.metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, resources.ListOptions(res.Labels))
	if err != nil {
		return 0, 0
	}

	var (
		totalCPU int64
		totalMem int64
		count    int
	)

	for _, podMetrics := range podMetricsList.Items {
		podName := podMetrics.Name

		if !m.podBelongsToWorkload(podName, res.Kind, res.Name) {
			continue
		}

		for _, containerMetrics := range podMetrics.Containers {
			if containerMetrics.Name != res.Container {
				continue
			}

			if cpu, ok := containerMetrics.Usage[v1.ResourceCPU]; ok {
				totalCPU += cpu.MilliValue()
			}

			if mem, ok := containerMetrics.Usage[v1.ResourceMemory]; ok {
				totalMem += mem.Value()
			}

			count++
		}
	}

	if count == 0 {
		return 0, 0
	}

	return totalCPU / int64(count), totalMem / int64(count)
}

// getVPAMetrics retrieves CPU and memory recommendations from VPA
func (m *Client) getVPAMetrics(ctx context.Context, namespace string, res resources.ResourceInfo) (int64, int64) {
	vpaList, err := m.vpaClient.AutoscalingV1().VerticalPodAutoscalers(namespace).List(ctx, resources.ListOptions(res.Labels))
	if err != nil {
		return 0, 0
	}

	for _, vpaItem := range vpaList.Items {
		if vpaItem.Spec.TargetRef != nil && vpaItem.Spec.TargetRef.Name == res.Name && vpaItem.Spec.TargetRef.Kind == res.Kind {
			if vpaItem.Status.Recommendation != nil {
				for _, containerMetrics := range vpaItem.Status.Recommendation.ContainerRecommendations {
					if containerMetrics.ContainerName == res.Container {
						var cpuUsage, memUsage int64

						if containerMetrics.Target != nil {
							if cpu, ok := containerMetrics.Target[v1.ResourceCPU]; ok {
								cpuUsage = cpu.MilliValue()
							}

							if mem, ok := containerMetrics.Target[v1.ResourceMemory]; ok {
								memUsage = mem.Value()
							}
						}

						return cpuUsage, memUsage
					}
				}
			}
		}
	}

	return 0, 0
}

// podBelongsToWorkload reports whether a pod name belongs to the given workload.
func (m *Client) podBelongsToWorkload(podName, kind, workloadName string) bool {
	switch kind {
	case "StatefulSet":
		// StatefulSet pods are named "<workload>-<ordinal>".
		return podName == workloadName || (len(podName) > len(workloadName)+1 && strings.HasPrefix(podName, workloadName+"-"))
	default:
		// Deployment, DaemonSet, CronJob/Job pods are named "<workload>-<hash>-<rand>"
		// or "<workload>-<rand>". Match by prefix.
		return podName == workloadName || (len(podName) > len(workloadName)+1 && strings.HasPrefix(podName, workloadName+"-"))
	}
}
