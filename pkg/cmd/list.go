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
	"strconv"
	"strings"

	"github.com/mgoltzsche/k8spkg/pkg/k8spkg"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	listCmd = &cobra.Command{
		Use:   "list",
		Short: "Lists the packages installed within the cluster",
		Long:  "Lists the packages installed within the cluster",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if len(args) != 0 {
				return errors.New("no arguments supported")
			}
			ctx := newContext()
			apiManager := k8spkg.NewPackageManager(kubeconfigFile)
			pkgs, err := apiManager.List(ctx, allNamespaces, namespace)
			if err != nil {
				return
			}
			nameLen := 7
			for _, pkg := range pkgs {
				if len(pkg.Name) > nameLen {
					nameLen = len(pkg.Name)
				}
			}
			lineFmt := "%-" + strconv.Itoa(nameLen) + "s    %s\n"
			fmt.Printf(lineFmt, "PACKAGE", "NAMESPACES")
			for _, pkg := range pkgs {
				fmt.Printf(lineFmt, pkg.Name, strings.Join(pkg.Namespaces, ","))
			}
			return
		},
	}
	allNamespaces bool
)

func init() {
	addRequestFlags(listCmd.Flags())
	listCmd.Flags().BoolVar(&allNamespaces, "all-namespaces", false, "Query all namespaces for packages")
	rootCmd.AddCommand(listCmd)
}
