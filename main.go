package main

import (
	"fmt"
	"os"

	"github.com/mgoltzsche/k8spkg/pkg/model"
	"github.com/mgoltzsche/k8spkg/pkg/state"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	verbose := false
	prune := false

	app := cli.NewApp()
	app.Name = "k8pkg"
	app.Version = version
	app.Usage = "A CLI to manage kubernetes API objects in packages"
	app.Author = "Max Goltzsche"
	app.EnableBashCompletion = true
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("version: %s\ncommit: %s\ndate: %s\n", c.App.Version, commit, date)
	}
	app.Flags = []cli.Flag{
		cli.BoolTFlag{
			Name:        "debug, d",
			Usage:       "Enable debug log",
			Destination: &verbose,
		},
	}
	app.Commands = []cli.Command{
		{
			Name:        "apply",
			Description: "Applies the provided package(s) waiting for required apiServices to become available",
			Usage:       "Applies a package",
			UsageText:   "k8spkg apply [--prune] SOURCE...",
			Flags: []cli.Flag{
				cli.BoolTFlag{
					Name:        "prune",
					Usage:       "Deletes all sources that belong to the provided package(s) but were not present within the input",
					Destination: &prune,
				},
			},
			Action: func(c *cli.Context) (err error) {
				pkg, err := loadPackage(c)
				if err != nil {
					return
				}
				mngr := state.NewPackageManager()
				return mngr.Apply(pkg, prune)
			},
		},
		{
			Name:        "delete",
			Description: "Deletes the provided packages from the k8s cluster",
			Usage:       "Deletes a package",
			UsageText:   "k8spkg delete PKG",
			Action: func(c *cli.Context) (err error) {
				if c.NArg() != 1 {
					return cli.NewExitError("no package to delete provided", 1)
				}
				mngr := state.NewPackageManager()
				return mngr.Delete(c.Args()[0])
			},
		},
		{
			Name:        "state",
			Description: "Returns the state of the provided package within the cluster",
			Usage:       "Prints a package's state within the cluster",
			UsageText:   "k8spkg state PKG",
			Action: func(c *cli.Context) (err error) {
				if c.NArg() != 1 {
					return cli.NewExitError("no package name provided", 1)
				}
				mngr := state.NewPackageManager()
				obj, err := mngr.State(c.Args()[0])
				if err != nil {
					return
				}
				return model.K8sPackage(obj).WriteYaml(os.Stdout)
			},
		},
		{
			Name:        "manifest",
			Description: "Prints the merged and labeled manifest",
			Usage:       "Prints a rendered package manifest",
			UsageText:   "k8spkg manifest SOURCE",
			Action: func(c *cli.Context) (err error) {
				pkg, err := loadPackage(c)
				if err != nil {
					return
				}
				return pkg.WriteYaml(os.Stdout)
			},
		},
	}
	logrus.SetLevel(logrus.DebugLevel)
	if err := app.Run(os.Args); err != nil {
		logrus.Fatalf("k8spkg: %s", err)
	}
}

func loadObjects(src string) (o []*model.K8sObject, err error) {
	baseDir, err := os.Getwd()
	if err != nil {
		return
	}
	return model.Objects(src, baseDir)
}

func loadPackage(c *cli.Context) (pkg model.K8sPackage, err error) {
	if c.NArg() != 1 {
		return nil, cli.NewExitError("missing SOURCE argument", 1)
	}
	// TODO: provide source as options (as in kubectl apply -f -r -k)
	o, err := loadObjects(c.Args()[0])
	return model.K8sPackage(o), err
}
