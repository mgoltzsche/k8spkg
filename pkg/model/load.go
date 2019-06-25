package model

import (
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/hashicorp/go-getter"
	"github.com/pkg/errors"
)

// TODO: maybe remove since kustomize should always be used to load sources

func Objects(src, baseDir string) (o []*K8sObject, err error) {
	if len(baseDir) == 0 {
		if baseDir, err = os.Getwd(); err != nil {
			return
		}
	}
	file, tmp, err := loadSource(src, baseDir)
	if err != nil {
		return
	}
	if tmp {
		defer os.RemoveAll(file)
	}
	err = collectObjects(file, &o)
	return o, errors.Wrapf(err, "source %s", src)
}

func loadSource(src, baseDir string) (file string, remote bool, err error) {
	var dtctd string
	if dtctd, err = getter.Detect(src, baseDir, getter.Detectors); err != nil {
		return
	}
	var u *url.URL
	u, err = url.Parse(dtctd)
	if err != nil {
		return
	}
	path := u.Path
	if u.RawPath != "" {
		path = u.RawPath
	}
	if u.Scheme == "file" {
		file = path
	} else {
		if file, err = ioutil.TempDir("", "k8spkg-"+filepath.Base(path)+"-"); err != nil {
			return
		}
		os.RemoveAll(file)
		remote = true
		err = getter.GetAny(file, dtctd)
		err = errors.Wrapf(err, "source %s", src)
	}
	return
}

func collectObjects(file string, obj *[]*K8sObject) (err error) {
	si, err := os.Stat(file)
	if err != nil {
		return
	}
	collectFn := manifest2objects
	if si.IsDir() {
		collectFn = manifests2objects
		kustomizeFile := filepath.Join(file, "kustomization.yaml")
		if _, e := os.Stat(kustomizeFile); e == nil {
			collectFn = kustomize2objects
		}
	}
	return collectFn(file, obj)
}

func manifests2objects(dir string, obj *[]*K8sObject) (err error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return
	}
	if len(files) == 0 {
		return errors.Errorf("no yaml files contained within dir %s", dir)
	}
	sort.Strings(files)
	for _, file := range files {
		if err = manifest2objects(file, obj); err != nil {
			return
		}
	}
	return nil
}

func manifest2objects(file string, obj *[]*K8sObject) (err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	o, err := K8sObjectsFromReader(f)
	*obj = append(*obj, o...)
	return
}

func kustomize2objects(dir string, obj *[]*K8sObject) (err error) {
	reader, writer := io.Pipe()
	defer func() {
		if e := reader.Close(); e != nil && err == nil {
			err = e
		}
		err = errors.Wrap(err, "render kustomization.yaml")
	}()
	errc := make(chan error)
	go func() {
		o, e := K8sObjectsFromReader(reader)
		*obj = append(*obj, o...)
		errc <- e
		writer.CloseWithError(e)
	}()
	c := exec.Command("kubectl", "kustomize", dir)
	c.Stdout = writer
	c.Stderr = os.Stderr
	err = c.Run()
	writer.Close()
	if e := <-errc; e != nil && err == nil {
		err = e
	}
	return
}
