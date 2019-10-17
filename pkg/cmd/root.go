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
	"github.com/mgoltzsche/k8spkg/pkg/client"
	"github.com/mgoltzsche/k8spkg/pkg/k8spkg"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	//homedir "github.com/mitchellh/go-homedir"
	//viper "github.com/spf13/viper"
)

var (
	debug          bool
	kubeconfigFile string
	clientFactory  = func(kubeconfigFile string) client.K8sClient {
		return client.NewK8sClient(kubeconfigFile) // replaced during test
	}
	//cfgFile string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:              "k8spkg",
	Short:            "Manages kubernetes API objects in packages",
	Long:             "Manages kubernetes API objects in packages",
	PersistentPreRun: applyLogLevel,
	SilenceUsage:     true,
	SilenceErrors:    true,
}

func pkgManager() *k8spkg.PackageManager {
	return k8spkg.NewPackageManager(k8sClient(), namespace)
}

func k8sClient() client.K8sClient {
	return clientFactory(kubeconfigFile)
}

func applyLogLevel(cmd *cobra.Command, args []string) {
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logrus.Fatalf("k8spkg: %s", err)
	}
}

func init() {
	//cobra.OnInitialize(initConfig)
	//rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.k8spkg.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug log")
	rootCmd.PersistentFlags().StringVar(&kubeconfigFile, "kubeconfig", "", "use a particular kubeconfig.yaml (overrides KUBECONFIG env var)")
}

/*
// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".cmd" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".k8spkg")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
*/
