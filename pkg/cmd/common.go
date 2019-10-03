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
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mgoltzsche/k8spkg/pkg/k8spkg"
	"github.com/mgoltzsche/k8spkg/pkg/kustomize"
	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/spf13/pflag"
)

var (
	timeout            time.Duration
	namespace          string
	sourceKustomize    string
	sourceFile         string
	pkgName            string
	enableAlphaPlugins bool
)

func addRequestFlags(f *pflag.FlagSet) {
	f.DurationVarP(&timeout, "timeout", "t", time.Duration(0), "Set command timeout")
	f.StringVarP(&namespace, "namespace", "n", "", "Sets the namespace to be used. If option -f or -k is provided the namespace is set on all namespaced input objects")
}

func addSourceFlags(f *pflag.FlagSet) {
	addRequestFlags(f)
	f.StringVarP(&sourceFile, "file", "f", "", "Load manifest from file or URL")
	f.StringVarP(&sourceKustomize, "kustomize", "k", "", "Load manifest from rendered kustomize source")
	f.BoolVar(&enableAlphaPlugins, "enable_alpha_plugins", false, "enable kustomize plugins (alpha feature)")
}

func addSourceNameFlags(f *pflag.FlagSet) {
	addSourceFlags(f)
	f.StringVar(&pkgName, "name", "", "Add package name label to all input objects")
}

func newContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		logrus.Debugf("received termination signal %s", sig)
		cancel()
	}()
	return ctx
}

func lookupPackage(ctx context.Context, args []string, pkgManager *k8spkg.PackageManager) (pkg *k8spkg.K8sPackage, err error) {
	if len(args) > 1 {
		return nil, errors.New("too many arguments provided")
	}
	if len(args) == 1 {
		// Load manifest from deployed package state
		if args[0] == "" {
			return nil, errors.New("empty package name argument provided")
		}
		if sourceKustomize != "" || sourceFile != "" {
			return nil, errors.New("package name argument and -f or -k option are mutually exclusive but both provided")
		}
		pkg, err = pkgManager.State(ctx, namespace, args[0])
	} else {
		// Load manifest from provided source
		pkg, err = sourcePackage(ctx)
	}
	return
}

func sourcePackage(ctx context.Context) (pkg *k8spkg.K8sPackage, err error) {
	reader, err := sourceReader(ctx)
	if err != nil {
		return
	}
	defer reader.Close()
	pkg, err = k8spkg.PkgFromManifest(reader, namespace, pkgName)
	return
}

func sourceReader(ctx context.Context) (io.ReadCloser, error) {
	if sourceKustomize != "" && sourceFile != "" {
		return nil, errors.New("options -f and -k are mutually exclusive but both provided")
	}
	if sourceKustomize != "" {
		return renderKustomize(sourceKustomize)
	} else if sourceFile != "" {
		return fileReader(ctx, sourceFile)
	}
	return nil, errors.New("no source: none of option -f or -k provided")
}

func renderKustomize(source string) (reader io.ReadCloser, err error) {
	reader, writer := io.Pipe()
	go func() {
		err := kustomize.Render(kustomize.RenderOptions{
			Source:             source,
			Out:                writer,
			EnableAlphaPlugins: enableAlphaPlugins,
		})
		writer.CloseWithError(err)
	}()
	return reader, nil
}

func fileReader(ctx context.Context, source string) (reader io.ReadCloser, err error) {
	if source == "-" { // read stdin
		reader = &noopReadCloser{os.Stdin}
	} else { // read file/dir
		var baseDir string
		if baseDir, err = os.Getwd(); err != nil {
			return
		}
		reader = resource.ManifestReader(ctx, source, baseDir)
	}
	return
}

type noopReadCloser struct {
	io.Reader
}

func (r *noopReadCloser) Close() error {
	return nil // do nothing, keep stdin open
}
