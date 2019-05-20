package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDescriptor(t *testing.T) {
	withDescriptor(t, "k8s.sources", func(d *SourceDescriptor) (err error) {
		if d.Name != "exampleproject" {
			t.Errorf("name (%q) != %s", d.Name, "exampleproject")
		}
		if d.DownloadDir != "downloads" {
			t.Errorf("downloadDir (%q) != %s", d.DownloadDir, "downloads")
		}
		if len(d.URLs) != 2 {
			t.Errorf("unexpected urls: %+v", d.URLs)
		}
		if len(d.Sources) != 2 {
			t.Errorf("unexpected sources: %+v", d.Sources)
		}
		return
	})
}

func TestClean(t *testing.T) {
	withDescriptor(t, "k8s.sources", func(d *SourceDescriptor) (err error) {
		dir := filepath.Dir(d.file)
		if err = os.MkdirAll(filepath.Join(dir, d.DownloadDir, "somesubdir"), 0755); err != nil {
			return
		}
		if err = d.Clean(); err != nil {
			return
		}
		if _, err = os.Stat(filepath.Join(dir, d.DownloadDir, "somesubdir")); err == nil || !os.IsNotExist(err) {
			return fmt.Errorf("dir still exists or error: %s", err)
		}
		return nil
	})
}

func TestDownload(t *testing.T) {
	withDescriptor(t, "k8s.sources", func(d *SourceDescriptor) (err error) {
		if len(d.URLs) < 1 {
			return fmt.Errorf("no urls within test data")
		}
		if err = d.DownloadURLs(); err != nil {
			return err
		}
		dir := filepath.Dir(d.file)
		for dest, _ := range d.URLs {
			if _, err = os.Stat(filepath.Join(dir, d.DownloadDir, dest)); err != nil {
				return err
			}
		}
		return nil
	})
}

func TestObjects(t *testing.T) {
	withDescriptor(t, "k8s.sources", func(d *SourceDescriptor) (err error) {
		if err = d.DownloadURLs(); err != nil {
			return
		}
		obj, err := d.Objects()
		if err != nil {
			return
		}
		cmDeployFound := false
		cmIssuerFound := false
		for _, o := range obj {
			if o.Kind() == "Deployment" && o.Metadata().Name == "cert-manager" {
				cmDeployFound = true
			}
			if o.Kind() == "ClusterIssuer" && o.Metadata().Name == "cluster-ca-issuer" {
				cmIssuerFound = true
			}
		}
		if !cmDeployFound {
			return fmt.Errorf("Did not find cert-manager in merged yaml")
		}
		if !cmIssuerFound {
			return fmt.Errorf("Did not find cluster-ca-issuer in merged yaml")
		}
		return nil
	})
}

func withDescriptor(t *testing.T, file string, fn func(*SourceDescriptor) error) {
	dir := "test"
	d, err := DescriptorFromFile(filepath.Join(dir, "k8s.sources"))
	if err != nil {
		t.Error("descriptor:", err)
		t.FailNow()
	}
	d.Clean()
	if err = fn(d); err != nil {
		d.Clean()
		t.Error(err)
		t.FailNow()
	}
}
