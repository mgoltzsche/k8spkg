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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	deleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Deletes a package",
		Long:  "Deletes the identified objects from the cluster and awaits their deletion",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if len(args) > 1 {
				return errors.New("too many arguments provided")
			}
			ctx := newContext()
			apiManager := pkgManager()
			if len(args) > 0 {
				// Find and delete objects by package name
				if sourceKustomize != "" || sourceFile != "" {
					return errors.New("package name argument and -f or -k option are mutually exclusive but both provided")
				}
				for _, pkgName := range args {
					if err = apiManager.Delete(ctx, pkgName); err != nil {
						return
					}
				}
				return
			}
			// Delete provided objects
			reader, err := sourceReader(ctx)
			if err != nil {
				return
			}
			defer reader.Close()
			obj, err := k8spkg.TransformedObjects(reader, namespace, "")
			if err != nil {
				return
			}
			// TODO: recover from wait error due to already removed object
			return apiManager.DeleteResources(ctx, obj.Refs())
		},
	}
)

func init() {
	addSourceFlags(deleteCmd.Flags())
	rootCmd.AddCommand(deleteCmd)
}
