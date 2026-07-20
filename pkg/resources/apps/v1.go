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

// Package apps parces the Helm release manifest and extracts resource information from Kubernetes standard resources for supported workloads.
package apps

import (
	"context"
	"fmt"
	"strings"

	"helm.sh/helm/v3/pkg/release"

	"github.com/sergelogvinov/helm-resources/pkg/metrics"
	"github.com/sergelogvinov/helm-resources/pkg/resources"
	crds "github.com/sergelogvinov/helm-resources/pkg/resources/crds"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/yaml"
)

const unknown = "unknown"

// ExtractResourcesFromHelmRelease extracts resource information from a Helm release manifest for supported workloads, including standard workloads and CRDs.
// nolint: cyclop,gocyclo
func ExtractResourcesFromHelmRelease(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	metricsClient *metrics.Client,
	release *release.Release,
) ([]resources.ResourceInfo, error) {
	var res []resources.ResourceInfo

	namespace := release.Namespace
	chartName := ""

	if release.Chart != nil && release.Chart.Metadata != nil {
		chartName = release.Chart.Metadata.Name
	}

	for doc := range strings.SplitSeq(release.Manifest, "---") {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var obj unstructured.Unstructured
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			continue // Skip invalid YAML
		}

		kind := obj.GetKind()
		apiVersion := obj.GetAPIVersion()

		standardWorkload := ((kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet") && apiVersion == "apps/v1") ||
			(kind == "CronJob" && apiVersion == "batch/v1")
		if !standardWorkload {
			resCRD, err := crds.ExtractResourcesFromCRD(ctx, clientset, metricsClient, release.Name, doc, namespace)
			if err != nil {
				continue
			}

			res = append(res, resCRD...)

			continue
		}

		var (
			containers   []v1.Container
			workloadName string
			replicas     string
			labels       map[string]string
		)

		switch kind {
		case "Deployment":
			var deployment appsv1.Deployment
			if err := yaml.Unmarshal([]byte(doc), &deployment); err != nil {
				continue
			}

			containers = deployment.Spec.Template.Spec.Containers
			workloadName = deployment.Name

			deployObj, err := clientset.AppsV1().Deployments(namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
			if err != nil {
				replicas = unknown
			} else {
				replicas = fmt.Sprintf("%d", deployObj.Status.ReadyReplicas)
				labels = deployObj.Spec.Template.Labels
			}
		case "StatefulSet":
			var statefulSet appsv1.StatefulSet
			if err := yaml.Unmarshal([]byte(doc), &statefulSet); err != nil {
				continue
			}

			containers = statefulSet.Spec.Template.Spec.Containers
			workloadName = statefulSet.Name

			stsObj, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, statefulSet.Name, metav1.GetOptions{})
			if err != nil {
				replicas = unknown
			} else {
				replicas = fmt.Sprintf("%d", stsObj.Status.ReadyReplicas)
				labels = stsObj.Spec.Template.Labels
			}
		case "DaemonSet":
			var daemonSet appsv1.DaemonSet
			if err := yaml.Unmarshal([]byte(doc), &daemonSet); err != nil {
				continue
			}

			containers = daemonSet.Spec.Template.Spec.Containers
			workloadName = daemonSet.Name

			dsObj, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, daemonSet.Name, metav1.GetOptions{})
			if err != nil {
				replicas = unknown
			} else {
				replicas = fmt.Sprintf("%d", dsObj.Status.NumberReady)
				labels = dsObj.Spec.Template.Labels
			}
		case "CronJob":
			var cronJob batchv1.CronJob
			if err := yaml.Unmarshal([]byte(doc), &cronJob); err != nil {
				continue
			}

			containers = cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers
			workloadName = cronJob.Name

			cronJobObj, err := clientset.BatchV1().CronJobs(namespace).Get(ctx, cronJob.Name, metav1.GetOptions{})
			if err != nil {
				replicas = unknown
			} else {
				replicas = fmt.Sprintf("%d", len(cronJobObj.Status.Active))
				labels = cronJobObj.Spec.JobTemplate.Spec.Template.Labels
			}
		}

		labels = resources.FilterLabels(labels)

		for _, container := range containers {
			resInfo := resources.ResourceInfo{
				Chart:     chartName,
				Release:   release.Name,
				Kind:      kind,
				Name:      workloadName,
				Replicas:  replicas,
				Container: container.Name,
				Labels:    labels,
			}

			if container.Resources.Requests != nil {
				if cpu := container.Resources.Requests[v1.ResourceCPU]; !cpu.IsZero() {
					resInfo.CPURequest = cpu.MilliValue()
				}

				if mem := container.Resources.Requests[v1.ResourceMemory]; !mem.IsZero() {
					resInfo.MemRequest = mem.Value()
				}
			}

			if container.Resources.Limits != nil {
				if cpu := container.Resources.Limits[v1.ResourceCPU]; !cpu.IsZero() {
					resInfo.CPULimit = cpu.MilliValue()
				}

				if mem := container.Resources.Limits[v1.ResourceMemory]; !mem.IsZero() {
					resInfo.MemLimit = mem.Value()
				}
			}

			cpuUsage, memUsage := metricsClient.GetContainerMetrics(ctx, namespace, resInfo)

			resInfo.CPUUsage = cpuUsage
			resInfo.MemUsage = memUsage

			res = append(res, resInfo)
		}
	}

	return res, nil
}
