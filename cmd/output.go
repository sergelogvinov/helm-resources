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

const none = "<none>"

func outputTable(resources []resources.ResourceInfo) error {
	fmt.Printf("\nRESOURCES:\n\n")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "KIND\tNAME\tREPLICAS\tCONTAINER\tREQUESTS (CPU/MEM)\tLIMITS (CPU/MEM)\tUSAGE (CPU/MEM)\n")

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

func outputTableRecommendations(recommendations []resources.ResourceRecommendation) error {
	if len(recommendations) > 0 {
		fmt.Printf("\nRESOURCE RECOMMENDATIONS:\n\n")

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "KIND\tNAME\tCONTAINER\tREQUESTS (CPU/Memory)\tLIMITS (CPU/Memory)\tCURRENT (CPU/Memory)\n")

		for _, rec := range recommendations {
			requestsInfo := formatResourceValues(rec.RecommendedCPURequest, rec.RecommendedMemRequest)
			limitsInfo := formatResourceValues(rec.RecommendedCPULimit, rec.RecommendedMemLimit)
			usageInfo := formatResourceValues(rec.CPUUsage, rec.MemUsage)

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				rec.Kind,
				rec.Name,
				rec.Container,
				requestsInfo,
				limitsInfo,
				usageInfo)
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
