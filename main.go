/*
Copyright 2025 Serge Logvinov.

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

// Package main is the entry point for the helm-resources CLI application.
package main

import (
	"errors"
	"os"

	"github.com/sergelogvinov/helm-resources/cmd"

	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/exec"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

func main() {
	if err := cmd.Run(); err != nil {
		var cmdErr cmd.Error
		switch {
		case errors.As(err, &cmdErr):
			os.Exit(cmdErr.Code)
		default:
			os.Exit(1)
		}
	}
}
