package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/urfave/cli"
	"gopkg.in/yaml.v3"
)

// TODO: look for dependent apiServices that were declared in previous packages
// -> wait for those and all pods of these packages
// -> treat all sources recursively as one package (required for kubectl apply --prune to work)
// -> support optional dependency list (added before actual sources)
//    and wait for required apiservices to become available
//    (BUT DON'T INTRODUCE UNNECESSARY DEPENDENCIES to base services that may be provided by the infrastructure)
func main() {
	app := cli.NewApp()
	app.Name = "k8src"
	app.Author = "Max Goltzsche"
	app.Flags = []cli.Flag{
		cli.BoolTFlag{
			Name:  "prune",
			Usage: "Deletes all sources that are labeled with the provided name but not occuring within the applied manifest",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:  "apply",
			Usage: "Applies the input packages waiting for required apiServices patiently",
			Action: func(c *cli.Context) error {
				if c.NArg() != 1 {
					return cli.NewExitError("1 argument required", 1)
				}
				panic("TODO: apply")
			},
		},
		{
			Name:  "manifest",
			Usage: "Prints the merged manifest",
			Action: func(c *cli.Context) error {
				if c.NArg() != 2 {
					return cli.NewExitError("2 arguments required", 1)
				}
				return renderManifest(c.Args()[0], c.Args()[1])
			},
		},
		{
			Name:  "fetch",
			Usage: "Fetches all remote sources",
			Action: func(c *cli.Context) (err error) {
				d, err := loadDescriptor(c)
				if err != nil {
					return
				}
				return d.DownloadURLs()
			},
		},
		{
			Name:  "clean",
			Usage: "Removes the download directory",
			Action: func(c *cli.Context) (err error) {
				d, err := loadDescriptor(c)
				if err != nil {
					return
				}
				return d.Clean()
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func loadDescriptor(c *cli.Context) (d *SourceDescriptor, err error) {
	if c.NArg() > 1 {
		return nil, cli.NewExitError("at most 1 argument expected", 1)
	}
	var dir string
	if c.NArg() == 1 {
		dir = c.Args()[0]
	} else {
		dir, err = os.Getwd()
		if err != nil {
			return
		}
	}
	return DescriptorFromFile(filepath.Join(dir, "k8s.src"))
}

func renderManifest(dir, name string) (err error) {
	d, err := DescriptorFromFile(DescriptorFile(dir))
	if err != nil {
		return
	}
	str, _ := yaml.Marshal(d)
	fmt.Println(string(str))
	return nil
}
