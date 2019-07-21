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
	"os"

	"github.com/mgoltzsche/k8spkg/pkg/k8spkg"
	"github.com/mgoltzsche/k8spkg/pkg/model"
	"github.com/spf13/cobra"
)

var (
	manifestCmd = &cobra.Command{
		Use:   "manifest",
		Short: "Prints a rendered package manifest",
		Long:  "Prints the merged and labeled manifest",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctx := newContext()
			apiManager := k8spkg.NewPackageManager(kubeconfigFile)
			pkg, err := lookupPackage(ctx, args, apiManager)
			if err != nil {
				return
			}
			return model.WriteManifest(pkg.Objects, os.Stdout)
		},
	}
)

func init() {
	addSourceNameFlags(manifestCmd.Flags())
	rootCmd.AddCommand(manifestCmd)
}
