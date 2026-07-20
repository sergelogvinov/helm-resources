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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/multierr"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"

	"github.com/sergelogvinov/helm-resources/pkg/metrics"
	"github.com/sergelogvinov/helm-resources/pkg/patch"
	"github.com/sergelogvinov/helm-resources/pkg/recommend"
	"github.com/sergelogvinov/helm-resources/pkg/resources"
	apps "github.com/sergelogvinov/helm-resources/pkg/resources/apps"

	"k8s.io/client-go/kubernetes"
)

const globalUsage = `
Show resource requests and limits for all workloads in a helm release.

This command analyzes a deployed helm release and displays the CPU and memory
requests and limits for all deployments, statefulsets, daemonsets, and cronjobs managed by the release.
`

func newResourcesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resources [RELEASE] [flags]",
		Short: "Show workload resources",
		Long:  globalUsage,
		Example: strings.Join([]string{
			"  helm resources my-release",
			"  helm resources my-release --namespace production",
			"  helm resources my-release --output json",
			"  helm resources my-release --values values.yaml",
		}, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: runResources,
	}

	cmd.Flags().StringP("namespace", "n", "", "namespace of the release")
	cmd.Flags().StringP("output", "o", "table", "output format (table, json, yaml)")
	cmd.Flags().StringArrayP("values", "f", []string{}, "Apply recommendations to values.yaml file (can be specified multiple times)")
	cmd.Flags().String("prometheus-url", "", "Prometheus server URL for metrics (e.g., http://prometheus:9090)")
	cmd.Flags().String("metrics-window", "5m", "Time window for metrics queries (e.g., 5m, 1h, 24h)")
	cmd.Flags().String("aggregation", "avg", "Aggregation function for metrics (avg, max)")

	return cmd
}

func runResources(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	flags := cmd.Flags()

	releaseName := args[0]
	namespace, _ := flags.GetString("namespace")       //nolint: errcheck
	outputFormat, _ := flags.GetString("output")       //nolint: errcheck
	applyToValues, _ := flags.GetStringArray("values") //nolint: errcheck

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

	metricsClient, err := metrics.New(prometheusURL, metricsWindow, aggregation, config)
	if err != nil {
		return fmt.Errorf("failed to create metrics client: %w", err)
	}

	resInfos, err := apps.ExtractResourcesFromHelmRelease(ctx, clientset, metricsClient, release)
	if err != nil {
		return fmt.Errorf("failed to extract resources: %w", err)
	}

	recommendations := recommend.AnalyzeRecommendations(resInfos)
	if len(applyToValues) > 0 && len(recommendations) > 0 {
		if err := applyRecommendationsToValuesFiles(recommendations, applyToValues); err != nil {
			return err
		}

		return nil
	}

	var errs error

	switch outputFormat {
	case "json":
		if err = outputJSON(resInfos); err != nil {
			errs = multierr.Append(errs, err)
		}
	case "yaml":
		if err = outputYAML(resInfos); err != nil {
			errs = multierr.Append(errs, err)
		}
	default:
		if err = outputTable(resInfos); err != nil {
			errs = multierr.Append(errs, err)
		}

		if err = outputTableRecommendations(recommendations); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	return errs
}

func applyRecommendationsToValuesFiles(recommendations []resources.ResourceRecommendation, valuesFiles []string) error {
	if len(recommendations) == 0 {
		return nil
	}

	var (
		retryRecommendations []resources.ResourceRecommendation
		errs                 error
	)

	maxRetries := len(valuesFiles)

	for retry, path := range valuesFiles {
		valuesData, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read values file: %w", err)
		}

		originalText := string(valuesData)
		updatedText := originalText

		retryRecommendations = []resources.ResourceRecommendation{}

		for _, r := range recommendations {
			newText, err := patch.ApplyPatchesToYaml(updatedText, r)
			if err != nil {
				if !errors.Is(err, patch.ErrNotFound) {
					return err
				}

				if retry == maxRetries-1 {
					errs = multierr.Append(errs, fmt.Errorf("failed to apply recommendation for %s/%s: %w", r.Kind, r.Name, err))
				}

				retryRecommendations = append(retryRecommendations, r)
			}

			updatedText = newText
		}

		if updatedText != originalText {
			if err := os.WriteFile(path, []byte(updatedText), 0o644); err != nil {
				return fmt.Errorf("failed to write updated values file %s: %w", path, err)
			}
		}

		recommendations = retryRecommendations
		if len(recommendations) == 0 {
			break
		}
	}

	return errs
}
