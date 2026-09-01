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
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

const (
	flagNamespace           = "namespace"
	flagOutput              = "output"
	flagValues              = "values"
	flagPrometheusURL       = "prometheus-url"
	envPrometheusURL        = "PROMETHEUS_URL"
	flagMetricsWindow       = "metrics-window"
	envMetricsWindow        = "METRICS_WINDOW"
	flagAggregation         = "aggregation"
	envAggregation          = "AGGREGATION"
	flagShowStats           = "show-stats"
	flagShowRecommendations = "show-recommendations"
	flagRecommendations     = "recommendations"
	flagNoHeaders           = "no-headers"
)

// Flags represents the command-line flags for the helm-resources command.
type Flags struct {
	Namespace           string
	Output              string
	Values              []string
	PrometheusURL       string
	MetricsWindow       string
	Aggregation         string
	ShowStats           bool
	ShowRecommendations bool
	Recommendations     string
	NoHeaders           bool
}

// DefaultFlags returns the default flags for the command.
func DefaultFlags() *Flags {
	return &Flags{
		ShowStats:           true,
		ShowRecommendations: true,
		NoHeaders:           false,
	}
}

// AddFlags adds the command-line flags to the provided FlagSet.
func (f *Flags) AddFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&f.Namespace, flagNamespace, "n", "", "namespace of the release")
	flags.StringVarP(&f.Output, flagOutput, "o", "table", "output format (table, json, yaml)")
	flags.StringArrayVarP(&f.Values, flagValues, "f", []string{}, "Apply recommendations to values.yaml file (can be specified multiple times)")

	// Prometheus and metrics aggregation flags
	flags.StringVar(&f.PrometheusURL, flagPrometheusURL, withDefaultString(envPrometheusURL, ""), "Prometheus server URL for metrics (e.g., http://prometheus:9090)")
	flags.StringVar(&f.MetricsWindow, flagMetricsWindow, withDefaultString(envMetricsWindow, "1h"), "Time window for metrics queries (e.g., 5m, 1h, 24h)")
	flags.StringVar(&f.Aggregation, flagAggregation, withDefaultString(envAggregation, "avg"), "Aggregation function for metrics (avg, max)")

	// Output formatting flags
	flags.BoolVar(&f.ShowStats, flagShowStats, f.ShowStats, "Show resource statistics")
	flags.BoolVar(&f.ShowRecommendations, flagShowRecommendations, f.ShowRecommendations, "Show resource recommendations")
	flags.StringVarP(&f.Recommendations, flagRecommendations, "r", "lo,hi", "Filter recommendations: 'lo' (lower resources needed), 'hi' (higher resources needed), or comma-separated")
	flags.BoolVar(&f.NoHeaders, flagNoHeaders, f.NoHeaders, "Do not print table headers")
}

func withDefaultString(key string, def string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		return def
	}

	return val
}

// ParseRecommendations parses the recommendations flag value and returns a map of recommendation types.
// Valid values are 'lo' (lower resources needed) and 'hi' (higher resources needed).
func (f *Flags) ParseRecommendations() (map[string]bool, error) {
	if f.Recommendations == "" {
		return nil, nil
	}

	recommendations := map[string]bool{
		"lo": false,
		"hi": false,
	}

	for t := range strings.SplitSeq(f.Recommendations, ",") {
		trimmed := strings.TrimSpace(strings.ToLower(t))
		if trimmed == "lo" || trimmed == "hi" {
			recommendations[trimmed] = true
		} else {
			return nil, fmt.Errorf("invalid recommendation type: %s (valid: 'lo', 'hi', or comma-separated 'lo,hi')", t)
		}
	}

	return recommendations, nil
}
