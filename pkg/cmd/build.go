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
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	buildCmd = &cobra.Command{
		Use:   "build",
		Short: "Renders a package manifest to stdout",
		Long:  "Renders a labeled package manifest to stdout",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if len(args) != 0 {
				return fmt.Errorf("no arguments supported but provided %+v", args)
			}
			ctx := newContext()
			pkg, err := sourcePackage(ctx)
			if err != nil {
				return
			}
			return pkg.Resources.WriteYaml(os.Stdout)
		},
	}
)

func init() {
	addSourceNameFlags(buildCmd.Flags())
	rootCmd.AddCommand(buildCmd)
}
