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

// CommandOptions represents the options of the command.
type CommandOptions struct {
	Flags *Flags
}

func newResourcesCommand() *cobra.Command {
	opts := CommandOptions{
		Flags: DefaultFlags(),
	}

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
		RunE: opts.RunResources,
	}

	opts.Flags.AddFlags(cmd.Flags())

	return cmd
}

// RunResources executes the helm-resources command.
func (o *CommandOptions) RunResources(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	settings := cli.New()
	if o.Flags.Namespace != "" {
		settings.SetNamespace(o.Flags.Namespace)
	}

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), func(_ string, _ ...any) {}); err != nil {
		return err
	}

	getAction := action.NewGet(actionConfig)

	release, err := getAction.Run(args[0])
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

	metricsClient, err := metrics.New(o.Flags.PrometheusURL, o.Flags.MetricsWindow, o.Flags.Aggregation, config)
	if err != nil {
		return fmt.Errorf("failed to create metrics client: %w", err)
	}

	resInfos, err := apps.ExtractResourcesFromHelmRelease(ctx, clientset, metricsClient, release)
	if err != nil {
		return fmt.Errorf("failed to extract resources: %w", err)
	}

	recommendationTypes, err := o.Flags.ParseRecommendations()
	if err != nil {
		return fmt.Errorf("failed to parse recommendations: %w", err)
	}

	var errs error

	recommendations := filterRecommendations(recommendationTypes, recommend.AnalyzeRecommendations(resInfos))
	if len(o.Flags.Values) > 0 && len(recommendations) > 0 {
		if err := applyRecommendationsToValuesFiles(recommendations, o.Flags.Values); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	switch o.Flags.Output {
	case "json":
		if err = outputJSON(resInfos); err != nil {
			errs = multierr.Append(errs, err)
		}
	case "yaml":
		if err = outputYAML(resInfos); err != nil {
			errs = multierr.Append(errs, err)
		}
	default:
		if o.Flags.ShowStats && len(resInfos) > 0 {
			if err = outputTable(o.Flags, resInfos); err != nil {
				errs = multierr.Append(errs, err)
			}
		}

		if o.Flags.ShowRecommendations && len(recommendations) > 0 {
			if err = outputTableRecommendations(o.Flags, recommendations); err != nil {
				errs = multierr.Append(errs, err)
			}
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

// filterRecommendations filters recommendations based on the recommendations flag.
// Valid values are 'lo' (lower resources needed) and 'hi' (higher resources needed).
func filterRecommendations(recommendationTypes map[string]bool, recommendations []resources.ResourceRecommendation) []resources.ResourceRecommendation {
	// If no recommendation types are specified, return all recommendations as-is.
	if len(recommendationTypes) == 0 {
		return recommendations
	}

	var filtered []resources.ResourceRecommendation

	for _, rec := range recommendations {
		shouldInclude := false

		if rec.CurrentCPURequest > 0 && rec.RecommendedCPURequest > 0 {
			switch {
			case recommendationTypes["lo"] && rec.CurrentCPURequest > rec.RecommendedCPURequest:
				shouldInclude = true
			case recommendationTypes["hi"] && rec.CurrentCPURequest < rec.RecommendedCPURequest:
				shouldInclude = true
			default:
				rec.RecommendedCPURequest = 0
			}
		}

		if rec.CurrentCPULimit > 0 && rec.RecommendedCPULimit > 0 {
			switch {
			case recommendationTypes["lo"] && rec.CurrentCPULimit > rec.RecommendedCPULimit:
				shouldInclude = true
			case recommendationTypes["hi"] && rec.CurrentCPULimit < rec.RecommendedCPULimit:
				shouldInclude = true
			default:
				rec.RecommendedCPULimit = 0
			}
		}

		if rec.CurrentMemRequest > 0 && rec.RecommendedMemRequest > 0 {
			switch {
			case recommendationTypes["lo"] && rec.CurrentMemRequest > rec.RecommendedMemRequest:
				shouldInclude = true
			case recommendationTypes["hi"] && rec.CurrentMemRequest < rec.RecommendedMemRequest:
				shouldInclude = true
			default:
				rec.RecommendedMemRequest = 0
			}
		}

		if rec.CurrentMemLimit > 0 && rec.RecommendedMemLimit > 0 {
			switch {
			case recommendationTypes["lo"] && rec.CurrentMemLimit > rec.RecommendedMemLimit:
				shouldInclude = true
			case recommendationTypes["hi"] && rec.CurrentMemLimit < rec.RecommendedMemLimit:
				shouldInclude = true
			default:
				rec.RecommendedMemLimit = 0
			}
		}

		if shouldInclude {
			filtered = append(filtered, rec)
		}
	}

	return filtered
}
