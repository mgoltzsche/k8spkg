package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mgoltzsche/k8spkg/pkg/k8spkg"
	"github.com/mgoltzsche/k8spkg/pkg/kustomize"
	"github.com/mgoltzsche/k8spkg/pkg/model"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		logrus.Fatalf("k8spkg: %s", err)
	}
}

func run(args []string, out io.Writer) error {
	var (
		debug         bool
		prune         bool
		allNamespaces bool
		timeout       time.Duration
	)

	apiManager := k8spkg.NewPackageManager()
	app := cli.NewApp()
	app.Name = "k8spkg"
	app.Version = version
	app.Usage = "A CLI to manage kubernetes API objects in packages"
	app.Author = "Max Goltzsche"
	app.EnableBashCompletion = true
	app.Before = func(c *cli.Context) error {
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("version: %s\ncommit: %s\ndate: %s\n", c.App.Version, commit, date)
	}
	app.Flags = []cli.Flag{
		cli.BoolTFlag{
			Name:        "debug, d",
			Usage:       "Enable debug log",
			Destination: &debug,
		},
	}
	timeoutFlag := cli.DurationFlag{
		Name:        "timeout, t",
		Usage:       "Set timeout",
		Destination: &timeout,
	}
	manifestFlags := []cli.Flag{
		cli.StringFlag{
			Name:  "f",
			Usage: "Load manifest from file or URL",
		},
		cli.StringFlag{
			Name:  "k",
			Usage: "Load manifest from rendered kustomize source",
		},
		timeoutFlag,
	}
	nameFlag := cli.StringFlag{
		Name:  "name",
		Usage: "Add package name label to all input objects",
	}
	namespaceFlag := cli.StringFlag{
		Name:  "namespace, n",
		Usage: "Add package name label to all input objects",
	}
	namedManifestFlags := append(manifestFlags, nameFlag, namespaceFlag)
	app.Commands = []cli.Command{
		{
			Name:        "apply",
			Description: "Installs or updates the provided source as package and waits for the rollout to succeed",
			Usage:       "Installs or updates a package",
			UsageText:   "k8spkg apply [-n <NAMESPACE>] [--name <PKG>] [--timeout <DURATION>] [--prune] {-f SRC|-k SRC}",
			Flags: append(namedManifestFlags,
				cli.BoolTFlag{
					Name:        "prune",
					Usage:       "Deletes all sources that belong to the provided package but were not present within the input",
					Destination: &prune,
				}),
			Action: func(c *cli.Context) (err error) {
				ctx := newContext(timeout)
				pkg, err := sourcePackage(ctx, inputOpts(c))
				if err != nil {
					return
				}
				return apiManager.Apply(ctx, pkg, prune)
			},
		},
		{
			Name:        "delete",
			Description: "Deletes the identified objects from the cluster and awaits their deletion",
			Usage:       "Deletes a package",
			UsageText:   "k8spkg delete [-n <NAMESPACE>] [--timeout <DURATION>] {-f SRC|-k SRC|PKG}",
			Flags:       append(manifestFlags, namespaceFlag),
			Action: func(c *cli.Context) (err error) {
				if c.NArg() > 1 {
					return cli.NewExitError("too many arguments provided", 1)
				}
				opts := inputOpts(c)
				ctx := newContext(timeout)
				if c.NArg() == 1 {
					// Find and delete objects by package name
					if opts.option != "" {
						return errors.Errorf("option -%s is not allowed when PACKAGE argument provided", opts.option)
					}
					return apiManager.Delete(ctx, opts.Namespace, c.Args()[0])
				}
				// Delete provided objects
				pkg, err := sourcePackage(ctx, opts)
				if err != nil {
					return
				}
				// TODO: recover from wait error due to already removed object
				return apiManager.DeleteObjects(ctx, pkg.Objects)
			},
		},
		{
			Name:        "list",
			Description: "Lists the packages installed within the cluster",
			Usage:       "Lists the packages installed within the cluster",
			UsageText:   "k8spkg [-n <NAMESPACE>] [--timeout <DURATION>] list",
			Flags: append([]cli.Flag{namespaceFlag, timeoutFlag},
				cli.BoolTFlag{
					Name:        "all-namespaces",
					Usage:       "Query all namespaces for packages",
					Destination: &allNamespaces,
				},
			),
			Action: func(c *cli.Context) (err error) {
				if c.NArg() != 0 {
					return cli.NewExitError("no arguments supported", 1)
				}
				ctx := newContext(timeout)
				opts := inputOpts(c)
				pkgs, err := apiManager.List(ctx, allNamespaces, opts.Namespace)
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
		},
		{
			Name:        "manifest",
			Description: "Prints the merged and labeled manifest",
			Usage:       "Prints a rendered package manifest",
			UsageText:   "k8spkg manifest [-n <NAMESPACE>] [--name <PKG>] [--timeout <DURATION>] {-f SRC|-k SRC|PKG}",
			Flags:       namedManifestFlags,
			Action: func(c *cli.Context) (err error) {
				ctx := newContext(timeout)
				pkg, err := lookupPackage(ctx, c, apiManager)
				if err != nil {
					return
				}
				return model.WriteManifest(pkg.Objects, out)
			},
		},
	}
	return app.Run(args)
}

type inputOptions struct {
	option    string
	source    string
	Name      string
	Namespace string
}

func inputOpts(c *cli.Context) (o inputOptions) {
	o.option = ""
	if c.IsSet("f") {
		o.option = "f"
		o.source = c.String("f")
	}
	if c.IsSet("k") {
		o.option += "k"
		o.source = c.String("k")
	}
	o.Name = c.String("name")
	o.Namespace = c.String("namespace")
	return
}

func newContext(timeout time.Duration) context.Context {
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

func lookupPackage(ctx context.Context, c *cli.Context, pkgManager *k8spkg.PackageManager) (pkg *k8spkg.K8sPackage, err error) {
	if c.NArg() > 1 {
		return nil, cli.NewExitError("too many arguments provided", 1)
	}
	opts := inputOpts(c)
	if c.NArg() == 1 {
		// Load manifest from deployed package state
		if c.Args()[0] == "" {
			return nil, errors.New("empty package name argument provided")
		}
		if opts.option != "" {
			return nil, errors.Errorf("package name and -%s option are exclusive but both provided", opts.option)
		}
		pkg, err = pkgManager.State(ctx, opts.Namespace, c.Args()[0])
	} else {
		// Load manifest from provided source
		pkg, err = sourcePackage(ctx, opts)
	}
	return
}

func sourcePackage(ctx context.Context, opt inputOptions) (pkg *k8spkg.K8sPackage, err error) {
	reader, err := sourceReader(ctx, opt)
	if err == nil {
		pkg, err = k8spkg.PkgFromManifest(reader, opt.Namespace, opt.Name)
	}
	return
}

func sourceReader(ctx context.Context, opt inputOptions) (io.Reader, error) {
	var readerFn func(context.Context, string) (io.Reader, error)
	switch opt.option {
	case "k":
		readerFn = renderKustomize
	case "f":
		readerFn = fileReader
	default:
		return nil, errors.New("exactly one of -f or -k must be provided")
	}
	if opt.source == "" {
		return nil, errors.Errorf("empty value provided to option -%s", opt.option)
	}
	return readerFn(ctx, opt.source)
}

func renderKustomize(ctx context.Context, source string) (reader io.Reader, err error) {
	reader, writer := io.Pipe()
	go func() {
		err := kustomize.Render(kustomize.RenderOptions{
			Source: source,
			Out:    writer,
		})
		writer.CloseWithError(err)
	}()
	return reader, nil
}

func fileReader(ctx context.Context, source string) (reader io.Reader, err error) {
	if source == "-" { // read stdin
		reader = os.Stdin
	} else { // read file/dir
		var baseDir string
		if baseDir, err = os.Getwd(); err != nil {
			return
		}
		reader = model.ManifestReader(ctx, source, baseDir)
	}
	return
}
