package labels

import (
	"bytes"
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/mgoltzsche/k8spkg/pkg/kustomize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddLabels(t *testing.T) {
	var opt *kustomize.RenderOptions
	expectedKustomization := `resources:
  - manifest.yaml
commonLabels:
  "key1": "value1"
  "key2": "value2"`
	render = func(o kustomize.RenderOptions) error {
		opt = &o
		assert.NotEmpty(t, opt.Source, "render source")
		assert.NotNil(t, opt.Out, "render writer")
		kFile := filepath.Join(opt.Source, "kustomization.yaml")
		mFile := filepath.Join(opt.Source, "manifest.yaml")
		kustomization, err := ioutil.ReadFile(kFile)
		require.NoError(t, err)
		manifest, err := ioutil.ReadFile(mFile)
		require.NoError(t, err)
		assert.Equal(t, "manifest", string(manifest), "manifest")
		assert.Equal(t, expectedKustomization, string(bytes.TrimSpace(kustomization)), "manifest")
		return nil
	}
	manifest := []byte("manifest")
	labels := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	var writer bytes.Buffer
	err := AddLabels(bytes.NewReader(manifest), labels, &writer)
	require.NoError(t, err, "AddLabels()")
	require.NotNil(t, opt, "render options")

	// error handling
	render = func(o kustomize.RenderOptions) error {
		return errors.New("errormock")
	}
	err = AddLabels(bytes.NewReader(manifest), labels, &writer)
	require.Error(t, err, "AddLabels()")
}
