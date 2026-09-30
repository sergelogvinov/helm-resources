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

package metrics

import (
	"fmt"
	"time"

	"github.com/sergelogvinov/helm-resources/pkg/resources"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	cacheddiscovery "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/metrics/pkg/client/custom_metrics"
)

// customMetricsTimeout bounds each custom-metrics request, the client
// methods accept no context.
const customMetricsTimeout = 15 * time.Second

var podGroupKind = schema.GroupKind{Kind: "Pod"}

// newCustomMetricsClient builds a client for the custom.metrics.k8s.io API
// served by kubernetes-custom-metrics gateway.
func newCustomMetricsClient(config *rest.Config) (custom_metrics.CustomMetricsClient, error) {
	cfg := rest.CopyConfig(config)
	cfg.Timeout = customMetricsTimeout

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(cacheddiscovery.NewMemCacheClient(discoveryClient))
	availableAPIs := custom_metrics.NewAvailableAPIsGetter(discoveryClient)

	return custom_metrics.NewForConfig(cfg, mapper, availableAPIs), nil
}

// getCustomMetrics retrieves historical CPU and memory usage from the
// custom metrics API (cpu_<stat>_<window>, memory_<stat>_<window>).
// The API serves per-pod values only (sum of all pod containers), so it is
// used only for single-container pods. Returns the per-pod average
// (or maximum with max aggregation) across the workload pods.
func (m *Client) getCustomMetrics(namespace string, res resources.ResourceInfo) (int64, int64) {
	if res.PodContainers != 1 {
		return 0, 0
	}

	selector := labels.SelectorFromSet(res.Labels)

	cpuByPod, err := m.getCustomPodMetric(namespace, res, selector, fmt.Sprintf("cpu_%s_%s", m.aggregation, m.metricsWindow), true)
	if err != nil || len(cpuByPod) == 0 {
		return 0, 0
	}

	memByPod, err := m.getCustomPodMetric(namespace, res, selector, fmt.Sprintf("memory_%s_%s", m.aggregation, m.metricsWindow), false)
	if err != nil || len(memByPod) == 0 {
		return 0, 0
	}

	return m.aggregate(cpuByPod), m.aggregate(memByPod)
}

// getCustomPodMetric returns the metric values of the workload pods keyed by pod name.
// CPU values are returned in millicores, memory in bytes.
func (m *Client) getCustomPodMetric(
	namespace string,
	res resources.ResourceInfo,
	selector labels.Selector,
	metric string,
	milli bool,
) (map[string]int64, error) {
	list, err := m.customMetricsClient.NamespacedMetrics(namespace).GetForObjects(podGroupKind, selector, metric, labels.Everything())
	if err != nil {
		return nil, err
	}

	values := make(map[string]int64, len(list.Items))

	for _, item := range list.Items {
		podName := item.DescribedObject.Name

		if !m.podBelongsToWorkload(podName, res.Kind, res.Name) {
			continue
		}

		if milli {
			values[podName] = item.Value.MilliValue()
		} else {
			values[podName] = item.Value.Value()
		}
	}

	return values, nil
}

// aggregate reduces per-pod values with the configured aggregation function.
func (m *Client) aggregate(values map[string]int64) int64 {
	var total, maxValue int64

	for _, v := range values {
		total += v
		maxValue = max(maxValue, v)
	}

	if m.aggregation == "max" {
		return maxValue
	}

	return total / int64(len(values))
}
