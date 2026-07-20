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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	namespace,
	kind,
	workloadName,
	containerName string,
) (int64, int64) {
	if m.prometheusClient != nil {
		return m.getPrometheusMetrics(ctx, namespace, workloadName, containerName)
	}

	if m.metricsClient != nil {
		return m.getKubernetesMetrics(ctx, namespace, kind, workloadName, containerName)
	}

	if m.vpaClient != nil {
		return m.getVPAMetrics(ctx, namespace, kind, workloadName, containerName)
	}

	return 0, 0
}

// getPrometheusMetrics retrieves CPU and memory usage from Prometheus
func (m *Client) getPrometheusMetrics(ctx context.Context, namespace, workloadName, containerName string) (int64, int64) {
	cpuQuery := fmt.Sprintf(`%s(rate(container_cpu_usage_seconds_total{namespace="%s",pod=~"%s.*",container="%s"}[%s])) * 1000`, m.aggregation, namespace, workloadName, containerName, m.metricsWindow)

	cpuResult, _, err := m.prometheusClient.Query(ctx, cpuQuery, time.Now())
	if err != nil {
		return 0, 0
	}

	memQuery := fmt.Sprintf(`%s(container_memory_usage_bytes{namespace="%s",pod=~"%s.*",container="%s"}[%s])`, m.aggregation, namespace, workloadName, containerName, m.metricsWindow)

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
func (m *Client) getKubernetesMetrics(ctx context.Context, namespace, kind, workloadName, containerName string) (int64, int64) {
	podMetricsList, err := m.metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
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

		if !m.podBelongsToWorkload(podName, kind, workloadName) {
			continue
		}

		for _, containerMetrics := range podMetrics.Containers {
			if containerMetrics.Name != containerName {
				continue
			}

			cpu := containerMetrics.Usage[v1.ResourceCPU]
			mem := containerMetrics.Usage[v1.ResourceMemory]

			totalCPU += cpu.MilliValue()
			totalMem += mem.Value()
			count++
		}
	}

	if count == 0 {
		return 0, 0
	}

	return totalCPU / int64(count), totalMem / int64(count)
}

// getVPAMetrics retrieves CPU and memory recommendations from VPA
func (m *Client) getVPAMetrics(ctx context.Context, namespace, kind, workloadName, containerName string) (int64, int64) {
	vpaList, err := m.vpaClient.AutoscalingV1().VerticalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, 0
	}

	for _, vpaItem := range vpaList.Items {
		if vpaItem.Spec.TargetRef != nil && vpaItem.Spec.TargetRef.Name == workloadName && vpaItem.Spec.TargetRef.Kind == kind {
			if vpaItem.Status.Recommendation != nil {
				for _, containerRec := range vpaItem.Status.Recommendation.ContainerRecommendations {
					if containerRec.ContainerName == containerName {
						var cpuUsage, memUsage int64

						if containerRec.Target != nil {
							if cpu, ok := containerRec.Target["cpu"]; ok {
								cpuUsage = cpu.MilliValue()
							}

							if mem, ok := containerRec.Target["memory"]; ok {
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
