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
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	v1prometheus "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"

	"github.com/sergelogvinov/helm-resources/pkg/metrics"
	"github.com/sergelogvinov/helm-resources/pkg/resources"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/yaml"
)

const (
	globalUsage = `
Show resource requests and limits for all workloads in a helm release.

This command analyzes a deployed helm release and displays the CPU and memory
requests and limits for all deployments, statefulsets, and daemonsets managed by the release.
`

	unknown = "unknown"
)

func newResourcesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resources [RELEASE] [flags]",
		Short: "Show workload resources",
		Long:  globalUsage,
		Example: strings.Join([]string{
			"  helm resources my-release",
			"  helm resources my-release --namespace production",
			"  helm resources my-release -o json",
		}, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: runResources,
	}

	cmd.Flags().StringP("namespace", "n", "", "namespace of the release")
	cmd.Flags().StringP("output", "o", "table", "output format (table, json, yaml)")
	cmd.Flags().String("prometheus-url", "", "Prometheus server URL for metrics (e.g., http://prometheus:9090)")
	cmd.Flags().String("metrics-window", "5m", "Time window for metrics queries (e.g., 5m, 1h, 24h)")
	cmd.Flags().String("aggregation", "avg", "Aggregation function for metrics (avg, max)")

	return cmd
}

func runResources(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	flags := cmd.Flags()

	releaseName := args[0]
	namespace, _ := flags.GetString("namespace")          //nolint: errcheck
	outputFormat, _ := flags.GetString("output")          //nolint: errcheck
	prometheusURL, _ := flags.GetString("prometheus-url") //nolint: errcheck
	metricsWindow, _ := flags.GetString("metrics-window") //nolint: errcheck
	aggregation, _ := flags.GetString("aggregation")      //nolint: errcheck

	settings := cli.New()
	if namespace != "" {
		settings.SetNamespace(namespace)
	}

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), func(_ string, _ ...any) {}); err != nil {
		return err
	}

	getAction := action.NewGet(actionConfig)

	release, err := getAction.Run(releaseName)
	if err != nil {
		return err
	}

	config, err := settings.RESTClientGetter().ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	var prometheusClient v1prometheus.API

	if prometheusURL != "" {
		promClient, err := api.NewClient(api.Config{
			Address: prometheusURL,
			RoundTripper: &http.Transport{
				IdleConnTimeout: 30 * time.Second,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create Prometheus client: %w", err)
		}

		prometheusClient = v1prometheus.NewAPI(promClient)
	}

	resources, err := extractResourcesFromManifest(ctx, release.Manifest, clientset, prometheusClient, settings.Namespace(), metricsWindow, aggregation)
	if err != nil {
		return fmt.Errorf("failed to extract resources: %w", err)
	}

	switch outputFormat {
	case "json":
		return outputJSON(resources)
	case "yaml":
		return outputYAML(resources)
	default:
		return outputTable(resources)
	}
}

func extractResourcesFromManifest(
	ctx context.Context,
	manifest string,
	clientset *kubernetes.Clientset,
	prometheusClient v1prometheus.API,
	namespace,
	metricsWindow,
	aggregation string,
) ([]resources.ResourceInfo, error) {
	var res []resources.ResourceInfo

	for doc := range strings.SplitSeq(manifest, "---") {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var obj unstructured.Unstructured
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			continue // Skip invalid YAML
		}

		kind := obj.GetKind()
		if (kind != "Deployment" && kind != "StatefulSet" && kind != "DaemonSet") || obj.GetAPIVersion() != "apps/v1" {
			continue
		}

		var (
			containers   []v1.Container
			workloadName string
			replicas     string
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
			}
		}

		for _, container := range containers {
			resInfo := resources.ResourceInfo{
				Kind:      kind,
				Name:      workloadName,
				Replicas:  replicas,
				Container: container.Name,
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

			cpuUsage, memUsage := metrics.GetContainerMetrics(ctx, prometheusClient, namespace, workloadName, container.Name, metricsWindow, aggregation)

			resInfo.CPUUsage = cpuUsage
			resInfo.MemUsage = memUsage

			res = append(res, resInfo)
		}
	}

	return res, nil
}
