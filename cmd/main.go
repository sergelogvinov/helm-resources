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

	"github.com/spf13/cobra"
)

const rootCmdLongUsage = `
The Helm Resources Plugin
`

// Run the root command for the helm-resources CLI application.
func Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resourcesCommand := newResourcesCommand()

	cmd := cobra.Command{
		Use:   "helm resources",
		Short: "Show deployment resources",
		Long:  rootCmdLongUsage,
		Args:  resourcesCommand.Args,
		RunE:  resourcesCommand.RunE,
	}

	cmd.Flags().AddFlagSet(resourcesCommand.Flags())
	cmd.AddCommand(newVersionCmd(), resourcesCommand)

	cmd.SetHelpCommand(&cobra.Command{}) // Disable the help command

	return cmd.ExecuteContext(ctx)
}
