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
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/sergelogvinov/helm-resources/pkg/resources"

	"sigs.k8s.io/yaml"
)

const none = "-"

func outputJSON(resources []resources.ResourceInfo) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	return encoder.Encode(resources)
}

func outputYAML(resources []resources.ResourceInfo) error {
	yamlData, err := yaml.Marshal(resources)
	if err != nil {
		return err
	}

	fmt.Print(string(yamlData))

	return nil
}

func outputTable(f *Flags, resources []resources.ResourceInfo) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if !f.NoHeaders {
		fmt.Fprintf(w, "KIND\tNAME\tREPLICAS\tCONTAINER\tREQUESTS (CPU/MEM)\tLIMITS (CPU/MEM)\tUSAGE (CPU/MEM)\n")
	}

	for _, res := range resources {
		requestsInfo := formatResourceValues(res.CPURequest, res.MemRequest)
		limitsInfo := formatResourceValues(res.CPULimit, res.MemLimit)
		usageInfo := formatResourceValues(res.CPUUsage, res.MemUsage)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			res.Kind,
			res.Name,
			res.Replicas,
			res.Container,
			requestsInfo,
			limitsInfo,
			usageInfo)
	}

	return w.Flush()
}

func outputTableRecommendations(f *Flags, recommendations []resources.ResourceRecommendation) error {
	if len(recommendations) > 0 {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		if !f.NoHeaders {
			if f.ShowStats {
				fmt.Printf("\nResource recommendations to adjust:\n\n")
			}

			fmt.Fprintf(w, "KIND\tNAME\tCONTAINER\tREQUESTS (CPU/MEM)\tREQUESTS DIFF (%%)\tLIMITS (CPU/MEM)\tLIMITS DIFF (%%)\tUSAGE (CPU/MEM)\n")
		}

		for _, rec := range recommendations {
			requestsInfo := formatResourceValues(rec.RecommendedCPURequest, rec.RecommendedMemRequest)
			requestsDiff := formatPercentageDiff(rec.CurrentCPURequest, rec.RecommendedCPURequest, rec.CurrentMemRequest, rec.RecommendedMemRequest)
			limitsInfo := formatResourceValues(rec.RecommendedCPULimit, rec.RecommendedMemLimit)
			limitsDiff := formatPercentageDiff(rec.CurrentCPULimit, rec.RecommendedCPULimit, rec.CurrentMemLimit, rec.RecommendedMemLimit)
			usageInfo := formatResourceValues(rec.CPUUsage, rec.MemUsage)

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				rec.Kind,
				rec.Name,
				rec.Container,
				requestsInfo,
				requestsDiff,
				limitsInfo,
				limitsDiff,
				usageInfo,
			)
		}

		return w.Flush()
	}

	return nil
}

func formatResourceValues(cpu, memory int64) string {
	cpuStr := formatCPU(cpu)
	memStr := formatMemory(memory)

	if cpuStr == none && memStr == none {
		return none
	}

	return fmt.Sprintf("%s/%s", cpuStr, memStr)
}

func formatCPU(milliCores int64) string {
	if milliCores == 0 {
		return none
	}

	if milliCores >= 1000 {
		return fmt.Sprintf("%.1f", float64(milliCores)/1000.0)
	}

	return fmt.Sprintf("%dm", milliCores)
}

func formatMemory(bytes int64) string {
	if bytes == 0 {
		return none
	}

	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.1fGi", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.0fMi", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.0fKi", float64(bytes)/1024)
	}

	return fmt.Sprintf("%d", bytes)
}

func formatPercentageDiff(currentCPU, recommendedCPU, currentMem, recommendedMem int64) string {
	cpuDiff := calculatePercentageDiff(currentCPU, recommendedCPU)
	memDiff := calculatePercentageDiff(currentMem, recommendedMem)

	if cpuDiff == none && memDiff == none {
		return none
	}

	return fmt.Sprintf("%s/%s", cpuDiff, memDiff)
}

func calculatePercentageDiff(current, recommended int64) string {
	if current == 0 || recommended == 0 {
		return none
	}

	diff := float64(recommended-current) / float64(current) * 100

	if diff > 0 {
		return fmt.Sprintf("+%.0f%%", diff)
	}

	if diff > -0.5 {
		return "0%"
	}

	return fmt.Sprintf("%.0f%%", diff)
}
