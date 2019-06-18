package main

import (
	"os"

	"github.com/mgoltzsche/k8spkg/pkg/model"
	"github.com/mgoltzsche/k8spkg/pkg/state"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// TODO: look for dependent apiServices that were declared in previous packages
// -> wait for those and all pods of these packages
// -> treat all sources recursively as one package (required for kubectl apply --prune to work)
// -> support optional dependency list (added before actual sources)
//    and wait for required apiservices to become available
//    (BUT DON'T INTRODUCE UNNECESSARY DEPENDENCIES to base services that may be provided by the infrastructure)
func main() {
	app := cli.NewApp()
	app.Name = "k8pkg"
	app.Author = "Max Goltzsche"
	app.Commands = []cli.Command{
		{
			Name:        "apply",
			Description: "Applies the provided package(s) waiting for required apiServices to become available",
			Usage:       "k8spkg apply [--prune] SOURCE...",
			Flags: []cli.Flag{
				cli.BoolTFlag{
					Name:  "prune",
					Usage: "Deletes all sources that belong to the provided package(s) but were not present within the input",
				},
				cli.BoolTFlag{
					Name:  "all",
					Usage: "Applies all dependencies as well",
				},
			},
			Action: func(c *cli.Context) (err error) {
				pkg, err := loadPackage(c)
				if err != nil {
					return
				}
				mngr := state.NewPackageManager()
				return mngr.Apply(pkg)
			},
		},
		{
			Name:        "delete",
			Description: "Deletes the provided packages from the k8s cluster",
			Usage:       "k8spkg delete PKG",
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
			Usage:       "k8spkg state PKG",
			Action: func(c *cli.Context) (err error) {
				if c.NArg() != 1 {
					return cli.NewExitError("no package name provided", 1)
				}
				mngr := state.NewPackageManager()
				obj, err := mngr.State(c.Args()[0])
				if err != nil {
					return
				}
				for _, o := range obj {
					if err = o.WriteYaml(os.Stdout); err != nil {
						return
					}
				}
				return
			},
		},
		{
			Name:        "manifest",
			Description: "Prints the merged and labeled manifest",
			Usage:       "k8spkg manifest PKGNAME SOURCEURL",
			Action: func(c *cli.Context) (err error) {
				pkg, err := loadPackage(c)
				if err != nil {
					return
				}
				return pkg.ToYaml(os.Stdout)
			},
		},
		{
			Name:  "fetch",
			Usage: "Fetches all remote sources",
			Action: func(c *cli.Context) (err error) {
				panic("TODO: fetch remote files into package dir (a way to describe where sources files originate from, can be converted and updated - helm support?)")
			},
		},
		{
			Name:  "clean",
			Usage: "Removes the download directory",
			Action: func(c *cli.Context) (err error) {
				panic("TODO: remove downloaded sources")
			},
		},
	}
	logrus.SetLevel(logrus.DebugLevel)
	if err := app.Run(os.Args); err != nil {
		logrus.Fatalf("k8spkg: %s", err)
	}
}

func loadObjects(src string) (o []model.K8sObject, err error) {
	baseDir, err := os.Getwd()
	if err != nil {
		return
	}
	return model.Objects(src, baseDir)
}

func loadPackage(c *cli.Context) (pkg *model.K8sPackage, err error) {
	if c.NArg() != 2 {
		return nil, cli.NewExitError("missing argument, required: PKGNAME SOURCEURL", 1)
	}
	o, err := loadObjects(c.Args()[1])
	return model.NewK8sPackage(c.Args()[0], o), err
}
