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
			nameLen := 7
			apps := [][2]string{}
			for evt := range pkgManager().List(ctx, namespace) {
				if evt.Err != nil {
					if err == nil {
						err = evt.Err
					}
					continue
				}
				if len(evt.App.Name) > nameLen {
					nameLen = len(evt.App.Name)
				}
				apps = append(apps, [2]string{evt.App.Name, evt.App.Namespace})
			}
			lineFmt := "%-" + strconv.Itoa(nameLen) + "s    %s\n"
			fmt.Printf(lineFmt, "APP", "NAMESPACE") // TODO: add kinds, ...?
			for _, app := range apps {
				fmt.Printf(lineFmt, app[0], app[1])
			}
			return
		},
	}
)

func init() {
	addRequestFlags(listCmd.Flags())
	rootCmd.AddCommand(listCmd)
}
