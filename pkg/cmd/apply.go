/*
Copyright © 2019 Max Goltzsche <max.goltzsche@gmail.com>

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
	"github.com/mgoltzsche/k8spkg/pkg/k8spkg"
	"github.com/spf13/cobra"
)

var (
	applyCmd = &cobra.Command{
		Use:   "apply",
		Short: "Installs or updates a package",
		Long: `Installs or updates the provided source as package
and waits for the rollout to complete`,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctx := newContext()
			apiManager := k8spkg.NewPackageManager()
			pkg, err := sourcePackage(ctx)
			if err != nil {
				return
			}
			return apiManager.Apply(ctx, pkg, prune)
		},
	}
	prune bool
)

func init() {
	addSourceNameFlags(applyCmd.Flags())
	applyCmd.Flags().BoolVar(&prune, "prune", false, "Deletes all sources that belong to the provided package but were not present within the input")
	rootCmd.AddCommand(applyCmd)
}
