package kustomize

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestRender(t *testing.T) {
	// Mock kubectl in case kubectl is used
	tmpBin, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer os.RemoveAll(tmpBin)
	err = os.Symlink("/proc/self/exe", filepath.Join(tmpBin, "kubectl"))
	require.NoError(t, err)
	os.Setenv("PATH", tmpBin+string(filepath.ListSeparator)+os.Getenv("PATH"))

	// Test
	var buf bytes.Buffer
	err = Render(RenderOptions{
		Source: "test/kustomizedpkg",
		//Source: "test/remote-base",
		//Source: "github.com/kubernetes-sigs/kustomize//examples/helloWorld?ref=v2.1.0",
		Out: &buf,
	})
	require.NoError(t, err, "Render()")
	rendered := rendered(t, buf.Bytes())
	names := containedNames(rendered)
	assert.Equal(t, []string{"mycert", "mydeployment"}, names, "rendered names")

	str := buf.String()
	pkgLabelPos := strings.Index(str, "  app.kubernetes.io/part-of: kustomizedpkg\n")
	assert.True(t, pkgLabelPos > 0, "app.kubernetes.io/part-of label should be contained")

	//
	// Test reject resources outside root directory
	//

	fileOutsideRoot := "/tmp/k8spkg-manifest-outside-root.yaml"
	tmpFile, err := os.OpenFile(fileOutsideRoot, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	require.NoError(t, err)
	defer os.Remove(fileOutsideRoot)
	f, err := os.Open(filepath.Join("test", "kustomizedpkg", "mycert.yaml"))
	require.NoError(t, err)
	_, err = io.Copy(tmpFile, f)
	require.NoError(t, err)
	tmpFile.Close()
	err = Render(RenderOptions{
		Source: "test/path-breach",
		Out:    &buf,
	})
	require.Error(t, err, "Render() outside project root")
	require.Contains(t, err.Error(), "security", "unexpected error message on path outside root directory: %s", err.Error())

	err = Render(RenderOptions{
		Source:       "test/path-breach",
		Out:          &buf,
		Unrestricted: true,
	})
	require.NoError(t, err, "Render() outside project root with RestrictRootDir=false")

	err = Render(RenderOptions{
		Source:       "test/uses-path-breach",
		Out:          &buf,
		Unrestricted: true,
	})
	require.Error(t, err, "Render() with transitive resource breaching root dir")
	require.Contains(t, err.Error(), "security", "unexpected error message on path outside root directory: %s", err.Error())
}

func containedNames(rendered []map[string]interface{}) (names []string) {
	for _, o := range rendered {
		m := o["metadata"]
		name := ""
		if mm, ok := m.(map[string]interface{}); ok {
			name = mm["name"].(string)
		} else {
			name = m.(map[interface{}]interface{})["name"].(string)
		}
		names = append(names, name)
	}
	return
}

func rendered(t *testing.T, rendered []byte) (r []map[string]interface{}) {
	dec := yaml.NewDecoder(bytes.NewReader(rendered))
	o := map[string]interface{}{}
	var err error
	for ; err == nil; err = dec.Decode(o) {
		require.NoError(t, err)
		if len(o) > 0 {
			r = append(r, o)
			o = map[string]interface{}{}
		}
	}
	if err != io.EOF {
		require.NoError(t, err)
	}
	return
}
