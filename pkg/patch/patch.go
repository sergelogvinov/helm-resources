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

package patch

import (
	"fmt"
	"strings"

	"github.com/sergelogvinov/helm-resources/pkg/resources"

	"sigs.k8s.io/yaml"
)

// WorkloadPath represents a path to a workload in the YAML structure
type WorkloadPath struct {
	Section   string // services, workers, jobs
	Workload  string // workload name
	Container string // container name (if applicable)
}

// ApplyPatchesToYaml applies resource recommendations to the given YAML text
// based on the provided values map to locate the correct paths.
func ApplyPatchesToYaml(yamlText string, res resources.ResourceRecommendation) (string, error) {
	var values map[string]any
	if err := yaml.Unmarshal([]byte(yamlText), &values); err != nil {
		return "", fmt.Errorf("failed to parse values file: %w", err)
	}

	workloadName, _ := strings.CutPrefix(res.Name, res.Release+"-")

	workloadPaths := findWorkloadPaths(values, workloadName)
	if len(workloadPaths) == 0 {
		return yamlText, fmt.Errorf("workload %s not found", workloadName)
	}

	for _, path := range workloadPaths {
		if newText, err := applyResourcePatchesToPath(yamlText, path, res); err == nil {
			yamlText = newText
		}
	}

	if !strings.HasSuffix(yamlText, "\n") {
		yamlText += "\n"
	}

	return yamlText, nil
}

func findWorkloadPaths(values map[string]any, workloadName string) []WorkloadPath {
	var paths []WorkloadPath

	for _, section := range []string{"services", "workers"} {
		if sectionData, ok := values[section].(map[string]any); ok {
			if workloadData, ok := sectionData[workloadName].(map[string]any); ok {
				if containers, ok := workloadData["containers"].([]any); ok {
					for i, container := range containers {
						if containerData, ok := container.(map[string]any); ok {
							containerName := fmt.Sprintf("container-%d", i)
							if name, ok := containerData["name"].(string); ok {
								containerName = name
							}

							paths = append(paths, WorkloadPath{
								Section:   section,
								Workload:  workloadName,
								Container: containerName,
							})
						}
					}
				} else {
					paths = append(paths, WorkloadPath{
						Section:  section,
						Workload: workloadName,
					})
				}
			}
		}
	}

	return paths
}

func applyResourcePatchesToPath(yamlText string, path WorkloadPath, rec resources.ResourceRecommendation) (string, error) {
	if rec.RecommendedCPULimit > 0 {
		if newText, err := applyValuePatch(yamlText, path, "limits", "cpu", formatCPUForYaml(rec.RecommendedCPULimit)); err == nil {
			yamlText = newText
		}
	}

	if rec.RecommendedMemLimit > 0 {
		if newText, err := applyValuePatch(yamlText, path, "limits", "memory", formatMemoryForYaml(rec.RecommendedMemLimit)); err == nil {
			yamlText = newText
		}
	}

	if rec.RecommendedCPURequest > 0 {
		if newText, err := applyValuePatch(yamlText, path, "requests", "cpu", formatCPUForYaml(rec.RecommendedCPURequest)); err == nil {
			yamlText = newText
		}
	}

	if rec.RecommendedMemRequest > 0 {
		if newText, err := applyValuePatch(yamlText, path, "requests", "memory", formatMemoryForYaml(rec.RecommendedMemRequest)); err == nil {
			yamlText = newText
		}
	}

	return yamlText, nil
}

func applyValuePatch(yamlText string, path WorkloadPath, resourceType, resource, newValue string) (string, error) {
	lines := strings.Split(yamlText, "\n")

	targetLine, targetIndent, err := findTargetLocation(lines, path)
	if err != nil {
		return "", err
	}

	resourcesLine, resourcesIndent := findResourcesSection(lines, targetLine, targetIndent)
	resourceTypeLine, resourceTypeIndent := findResourceTypeSection(lines, resourcesLine, resourcesIndent, resourceType)
	resourceLine := findResourceLine(lines, resourceTypeLine, resourceTypeIndent, resource)

	if resourceLine >= 0 {
		indent := len(lines[resourceLine]) - len(strings.TrimLeft(lines[resourceLine], " \t"))
		lines[resourceLine] = strings.Repeat(" ", indent) + resource + ": " + newValue
	} else {
		lines = addMissingResourceStructure(lines, targetLine, targetIndent, resourcesLine, resourcesIndent,
			resourceTypeLine, resourceTypeIndent, resourceType, resource, newValue)
	}

	return strings.Join(lines, "\n"), nil
}

func findTargetLocation(lines []string, path WorkloadPath) (int, int, error) {
	sectionFound := false
	workloadFound := false
	containerFound := path.Container == ""
	inContainers := false
	containerIndex := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		if !sectionFound && trimmed == path.Section+":" {
			sectionFound = true

			continue
		}

		if !sectionFound {
			continue
		}

		if !workloadFound && strings.HasPrefix(trimmed, path.Workload+":") {
			workloadFound = true

			if path.Container == "" {
				return i, indent, nil
			}

			continue
		}

		if !workloadFound {
			continue
		}

		if !inContainers && strings.HasPrefix(trimmed, "containers:") {
			inContainers = true
			containerIndex = -1

			continue
		}

		if path.Container != "" && !containerFound && inContainers {
			if strings.HasPrefix(trimmed, "- ") {
				containerIndex++
				if path.Container == fmt.Sprintf("container-%d", containerIndex) {
					return i, indent, nil
				}
			}

			if strings.Contains(line, "name: "+path.Container) {
				return i, indent, nil
			}

			if indent <= len("containers:") && trimmed != "" && !strings.HasPrefix(trimmed, "- ") && !strings.Contains(trimmed, "name:") {
				break
			}
		}
	}

	return -1, 0, fmt.Errorf("target location not found: %s.%s.%s", path.Section, path.Workload, path.Container)
}

func findResourcesSection(lines []string, startLine, baseIndent int) (int, int) {
	if startLine < 0 {
		return -1, 0
	}

	for i := startLine + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		if trimmed != "" && indent <= baseIndent {
			break
		}

		if strings.HasPrefix(trimmed, "resources:") {
			return i, indent
		}
	}

	return -1, 0
}

func findResourceTypeSection(lines []string, startLine, baseIndent int, resourceType string) (int, int) {
	if startLine < 0 {
		return -1, 0
	}

	for i := startLine + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		if trimmed != "" && indent <= baseIndent {
			break
		}

		if strings.HasPrefix(trimmed, resourceType+":") {
			return i, indent
		}
	}

	return -1, 0
}

func findResourceLine(lines []string, startLine, baseIndent int, resource string) int {
	if startLine < 0 {
		return -1
	}

	for i := startLine + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		if trimmed != "" && indent <= baseIndent {
			break
		}

		if strings.HasPrefix(trimmed, resource+":") {
			return i
		}
	}

	return -1
}

func addMissingResourceStructure(
	lines []string,
	targetLine,
	targetIndent int,
	resourcesLine,
	resourcesIndent int,
	resourceTypeLine,
	resourceTypeIndent int,
	resourceType,
	resource,
	newValue string,
) []string {
	var (
		insertLines []string
		insertPos   int
	)

	switch {
	case resourcesLine < 0:
		baseIndent := targetIndent + 2
		insertLines = []string{
			strings.Repeat(" ", baseIndent) + "resources:",
			strings.Repeat(" ", baseIndent+2) + resourceType + ":",
			strings.Repeat(" ", baseIndent+4) + resource + ": " + newValue,
		}
		insertPos = findInsertPosition(lines, targetLine, targetIndent)
	case resourceTypeLine < 0:
		baseIndent := resourcesIndent + 2
		insertLines = []string{
			strings.Repeat(" ", baseIndent) + resourceType + ":",
			strings.Repeat(" ", baseIndent+2) + resource + ": " + newValue,
		}
		insertPos = findInsertPosition(lines, resourcesLine, resourcesIndent)
	default:
		baseIndent := resourceTypeIndent + 2
		insertLines = []string{
			strings.Repeat(" ", baseIndent) + resource + ": " + newValue,
		}
		insertPos = findInsertPosition(lines, resourceTypeLine, resourceTypeIndent)
	}

	return append(lines[:insertPos], append(insertLines, lines[insertPos:]...)...)
}

func findInsertPosition(lines []string, startLine, baseIndent int) int {
	for i := startLine + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		if trimmed != "" && indent <= baseIndent {
			return i
		}
	}

	return len(lines)
}
