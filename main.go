package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/mgoltzsche/k8spkg/pkg/kustomize"
	"github.com/mgoltzsche/k8spkg/pkg/labels"
	"github.com/mgoltzsche/k8spkg/pkg/model"
	"github.com/mgoltzsche/k8spkg/pkg/state"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	version    = "dev"
	commit     = "none"
	date       = "unknown"
	apiManager = state.NewPackageManager()
)

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		logrus.Fatalf("k8spkg: %s", err)
	}
}

func run(args []string, out io.Writer) error {
	debug := false
	prune := false
	var timeout time.Duration

	app := cli.NewApp()
	app.Name = "k8pkg"
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
	app.Commands = []cli.Command{
		{
			Name:        "apply",
			Description: "Installs or updates the provided source as package waiting for successful rollout",
			Usage:       "Installs or updates a package",
			UsageText:   "k8spkg apply [--name <PKG>] [--timeout <DURATION>] [--prune] {-f SRC|-k SRC}",
			Flags: append(manifestFlags,
				nameFlag,
				cli.BoolTFlag{
					Name:        "prune",
					Usage:       "Deletes all sources that belong to the provided package but were not present within the input",
					Destination: &prune,
				}),
			Action: func(c *cli.Context) (err error) {
				ctx := newContext(timeout)
				reader, err := sourceReader(ctx, inputOpts(c))
				if err != nil {
					return
				}
				obj, err := model.FromReader(reader)
				if err != nil {
					return
				}
				return apiManager.Apply(ctx, obj, prune)
			},
		},
		{
			Name:        "delete",
			Description: "Deletes the provided packages from the k8s cluster",
			Usage:       "Deletes a package",
			UsageText:   "k8spkg delete [--timeout <DURATION>] {-f SRC|-k SRC|PKG}",
			Flags:       manifestFlags,
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
					return apiManager.Delete(ctx, c.Args()[0])
				}
				// Delete provided objects
				reader, err := sourceReader(ctx, opts)
				if err != nil {
					return
				}
				obj, err := model.FromReader(reader)
				if err != nil {
					return
				}
				// TODO: recover from wait error due to already removed object
				return apiManager.DeleteObjects(ctx, obj)
			},
		},
		{
			Name:        "list",
			Description: "Lists the packages installed within the cluster",
			Usage:       "Lists the packages installed within the cluster",
			UsageText:   "k8spkg [--timeout <DURATION>] list",
			Flags:       []cli.Flag{timeoutFlag},
			Action: func(c *cli.Context) (err error) {
				if c.NArg() != 0 {
					return cli.NewExitError("no arguments supported", 1)
				}
				ctx := newContext(timeout)
				objects, err := apiManager.State(ctx, "")
				if err != nil {
					return
				}
				pkgs := map[string]bool{}
				var pkgNames []string
				for _, o := range objects {
					if pkgName := o.Labels()[state.PKG_LABEL]; pkgName != "" {
						if !pkgs[pkgName] {
							pkgNames = append(pkgNames, pkgName)
							pkgs[pkgName] = true
						}
					}
				}
				sort.Strings(pkgNames)
				for _, pkgName := range pkgNames {
					fmt.Println(pkgName)
				}
				return
			},
		},
		{
			Name:        "manifest",
			Description: "Prints the merged and labeled manifest",
			Usage:       "Prints a rendered package manifest",
			UsageText:   "k8spkg manifest [--name <PKG>] [--timeout <DURATION>] {-f SRC|-k SRC|PKG}",
			Flags:       append(manifestFlags, nameFlag),
			Action: func(c *cli.Context) (err error) {
				ctx := newContext(timeout)
				obj, err := loadObjects(ctx, c)
				if err != nil {
					return
				}
				return model.WriteManifest(obj, out)
			},
		},
	}
	return app.Run(args)
}

type inputOptions struct {
	option string
	source string
	name   string
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
	o.name = c.String("name")
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

func loadObjects(ctx context.Context, c *cli.Context) (obj []*model.K8sObject, err error) {
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
		obj, err = apiManager.State(ctx, c.Args()[0])
	} else {
		// Load manifest from provided source
		var reader io.Reader
		if reader, err = sourceReader(ctx, opts); err != nil {
			return
		}
		obj, err = model.FromReader(reader)
	}
	if err == nil && len(obj) == 0 {
		err = errors.New("no objects contained")
	}
	return
}

func sourceReader(ctx context.Context, opt inputOptions) (reader io.Reader, err error) {
	valid := func() (err error) {
		if opt.source == "" {
			err = errors.Errorf("empty value provided to option -%s", opt.option)
		}
		return
	}
	switch opt.option {
	case "k":
		if err = valid(); err == nil {
			reader = renderKustomize(opt.source)
		}
	case "f":
		if opt.source == "-" {
			reader = os.Stdin
		} else {
			var baseDir string
			if baseDir, err = os.Getwd(); err != nil {
				return
			}
			if err = valid(); err == nil {
				reader = model.ManifestReader(ctx, opt.source, baseDir)
			}
		}
	default:
		err = errors.New("exactly one of -f or -k must be provided")
	}
	if err == nil && opt.name != "" {
		reader = setPackageName(reader, opt.name)
	}
	return
}

func renderKustomize(source string) io.Reader {
	reader, writer := io.Pipe()
	go func() {
		err := kustomize.Render(kustomize.RenderOptions{
			Source: source,
			Out:    writer,
		})
		writer.CloseWithError(err)
	}()
	return reader
}

func setPackageName(reader io.Reader, pkgName string) io.Reader {
	labeledReader, writer := io.Pipe()
	go func() {
		labelMap := map[string]string{state.PKG_LABEL: pkgName}
		writer.CloseWithError(labels.AddLabels(reader, labelMap, writer))
	}()
	return labeledReader
}
