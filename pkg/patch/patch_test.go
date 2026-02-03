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

package patch_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergelogvinov/helm-resources/pkg/patch"
	"github.com/sergelogvinov/helm-resources/pkg/resources"
)

const (
	simpleServiceYAML = `
someOtherField: someValue
services:
  backend:
    resources:
      requests:
        cpu: 100m
        memory: 768Mi
  myservice: &baseservice
    resources:
      limits:
        cpu: 500m
        memory: 1Gi
      requests:
        cpu: 100m
        memory: 768Mi
  mysecondservice:
    <<: *baseservice
`
	complexServiceYAML = `
someOtherField: someValue
workers:
  myservice:
    containers:
      - name: main
        resources:
          limits:
            cpu: 500m
            memory: 1Gi
          requests:
            cpu: 100m
            memory: 768Mi
      - name: sidecar
        resources:
          limits:
            cpu: 50m
            memory: 64Mi
          requests:
            cpu: 50m
            memory: 32Mi
`
)

func TestApplyPatchesToYaml(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		resources resources.ResourceRecommendation
		expect    string
		expectErr error
	}{
		{
			name: "service not found",
			yaml: simpleServiceYAML,
			resources: resources.ResourceRecommendation{
				Release:               "backend",
				Name:                  "not-found-service",
				RecommendedCPURequest: 100,
				RecommendedMemRequest: 256 * 1024 * 1024,
				RecommendedCPULimit:   200,
				RecommendedMemLimit:   512 * 1024 * 1024,
			},
			expectErr: fmt.Errorf("workload not-found-service not found"),
		},
		{
			name: "simple service patch",
			yaml: simpleServiceYAML,
			resources: resources.ResourceRecommendation{
				Release:               "backend",
				Name:                  "myservice",
				RecommendedCPURequest: 100,
				RecommendedMemRequest: 256 * 1024 * 1024,
				RecommendedCPULimit:   200,
				RecommendedMemLimit:   512 * 1024 * 1024,
			},
			expect: `
someOtherField: someValue
services:
  backend:
    resources:
      requests:
        cpu: 100m
        memory: 768Mi
  myservice: &baseservice
    resources:
      limits:
        cpu: 200m
        memory: 512Mi
      requests:
        cpu: 100m
        memory: 256Mi
  mysecondservice:
    <<: *baseservice
`,
		},
		{
			name: "simple service resources patch",
			yaml: simpleServiceYAML,
			resources: resources.ResourceRecommendation{
				Release:               "backend",
				Name:                  "mysecondservice",
				RecommendedCPURequest: 100,
				RecommendedMemRequest: 256 * 1024 * 1024,
				RecommendedCPULimit:   200,
				RecommendedMemLimit:   512 * 1024 * 1024,
			},
			expect: `
someOtherField: someValue
services:
  backend:
    resources:
      requests:
        cpu: 100m
        memory: 768Mi
  myservice: &baseservice
    resources:
      limits:
        cpu: 500m
        memory: 1Gi
      requests:
        cpu: 100m
        memory: 768Mi
  mysecondservice:
    <<: *baseservice

    resources:
      limits:
        cpu: 200m
        memory: 512Mi
      requests:
        cpu: 100m
        memory: 256Mi
`,
		},
		{
			name: "simple service resources limits patch",
			yaml: simpleServiceYAML,
			resources: resources.ResourceRecommendation{
				Release:               "backend",
				Name:                  "backend",
				RecommendedCPURequest: 100,
				RecommendedMemRequest: 256 * 1024 * 1024,
				RecommendedCPULimit:   200,
				RecommendedMemLimit:   512 * 1024 * 1024,
			},
			expect: `
someOtherField: someValue
services:
  backend:
    resources:
      requests:
        cpu: 100m
        memory: 256Mi
      limits:
        cpu: 200m
        memory: 512Mi
  myservice: &baseservice
    resources:
      limits:
        cpu: 500m
        memory: 1Gi
      requests:
        cpu: 100m
        memory: 768Mi
  mysecondservice:
    <<: *baseservice
`,
		},
		{
			name: "complex service patch",
			yaml: complexServiceYAML,
			resources: resources.ResourceRecommendation{
				Release:               "backend",
				Name:                  "myservice",
				Container:             "main",
				RecommendedCPURequest: 100,
				RecommendedMemRequest: 256 * 1024 * 1024,
				RecommendedCPULimit:   200,
				RecommendedMemLimit:   512 * 1024 * 1024,
			},
			expect: `
someOtherField: someValue
workers:
  myservice:
    containers:
      - name: main
        resources:
          limits:
            cpu: 200m
            memory: 512Mi
          requests:
            cpu: 100m
            memory: 256Mi
      - name: sidecar
        resources:
          limits:
            cpu: 50m
            memory: 64Mi
          requests:
            cpu: 50m
            memory: 32Mi
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := patch.ApplyPatchesToYaml(tt.yaml, tt.resources)

			if tt.expectErr != nil {
				assert.EqualError(t, err, tt.expectErr.Error())

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expect, res)
		})
	}
}
