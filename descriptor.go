package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

type SourceDescriptor struct {
	file        string
	Name        string            `yaml:"name,omitempty"`
	DownloadDir string            `yaml:"downloadDir,omitempty"`
	URLs        map[string]string `yaml:"urls,omitempty"`
	Sources     []string          `yaml:"sources,omitempty"`
}

func DescriptorFile(dir string) string {
	return filepath.Join(dir, "k8s.sources")
}

func DescriptorFromFile(file string) (r *SourceDescriptor, err error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	d := SourceDescriptor{
		file:        file,
		DownloadDir: "fetched",
	}
	if err = yaml.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("descriptor %s: %s", file, err)
	}
	return &d, nil
}

func (src *SourceDescriptor) DownloadURLs() (err error) {
	downloads, err := src.downloadMap()
	if err != nil {
		return
	}
	for dest, url := range downloads {
		if err = downloadURL(url, dest); err != nil {
			return fmt.Errorf("%s: %s", src.Name, err)
		}
	}
	return
}

func (src *SourceDescriptor) downloadMap() (d map[string]string, err error) {
	d = map[string]string{}
	for dest, url := range src.URLs {
		dest, err = src.projectFile(filepath.Join(src.DownloadDir, dest), "download destination")
		if err != nil {
			return
		}
		d[dest] = url
	}
	return
}

func (src *SourceDescriptor) Clean() (err error) {
	downloadDir, err := src.projectFile(src.DownloadDir, "download dir")
	if err != nil {
		return
	}
	if downloadDir == filepath.Dir(src.file) {
		return fmt.Errorf("Refused to delete %s: download dir == project dir", downloadDir)
	}
	return os.RemoveAll(downloadDir)
}

type objectCollector struct {
	objects []K8sObject
}

func (src *SourceDescriptor) Objects() (o K8sObjects, err error) {
	collector := objectCollector{[]K8sObject{}}
	err = src.collectObjects(&collector)
	return collector.objects, err
}

func (src *SourceDescriptor) collectObjects(collector *objectCollector) (err error) {
	for _, file := range src.Sources {
		if file, err = src.projectFile(file, "local source file"); err != nil {
			return
		}
		si, e := os.Stat(file)
		if e != nil {
			return fmt.Errorf("%s: source: %s", src.Name, e)
		}
		collectFn := collectFileObj
		if si.IsDir() {
			collectFn = includeDir
		}
		if err = collectFn(file, collector); err != nil {
			return
		}
	}
	return
}

func includeDir(dir string, collector *objectCollector) (err error) {
	descrFile := DescriptorFile(dir)
	if _, err = os.Stat(descrFile); err == nil {
		// child descriptor
		var d *SourceDescriptor
		if d, err = DescriptorFromFile(descrFile); err != nil {
			return
		}
		return d.collectObjects(collector)
	} else if !os.IsNotExist(err) {
		return
	}
	return collectDirObj(dir, collector)
}

func collectDirObj(dir string, collector *objectCollector) (err error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return
	}
	if len(files) == 0 {
		return fmt.Errorf("no yaml files contained within dir %s", dir)
	}
	sort.Strings(files)
	for _, file := range files {
		if err = collectFileObj(file, collector); err != nil {
			return
		}
	}
	return nil
}

func collectFileObj(file string, collector *objectCollector) (err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	dec := yaml.NewDecoder(f)
	o := map[string]interface{}{}
	for ; err == nil; err = dec.Decode(o) {
		if len(o) > 0 {
			collector.objects = append(collector.objects, K8sObject(o))
			o = map[string]interface{}{}
		}
	}
	if err == io.EOF {
		err = nil
	}
	return
}

func (src *SourceDescriptor) projectFile(file, name string) (absFile string, err error) {
	dir := filepath.Dir(src.file)
	absFile = filepath.Join(dir, file)
	if !filepath.HasPrefix(absFile, dir) {
		return "", fmt.Errorf("%s: %s %s is outside the project directory", src.Name, name, file)
	}
	return
}
