package transform

import (
	"bytes"
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/mgoltzsche/k8spkg/pkg/kustomize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestTransform(t *testing.T) {
	kustomization := func(dir string) (*TransformOptions, error) {
		return &TransformOptions{
			Resources:    []string{"manifest.yaml"},
			Namespace:    "mytestns",
			CommonLabels: map[string]string{"key1": "value1"},
		}, nil
	}
	var opt *kustomize.RenderOptions
	render = func(o kustomize.RenderOptions) error {
		opt = &o
		assert.NotEmpty(t, opt.Source, "render source")
		assert.NotNil(t, opt.Out, "render writer")
		kFile := filepath.Join(opt.Source, "kustomization.yaml")
		kustomizationYaml, err := ioutil.ReadFile(kFile)
		require.NoError(t, err)
		transformOpts, err := kustomization("")
		require.NoError(t, err)
		expectedKustomizationYaml, err := yaml.Marshal(transformOpts)
		require.NoError(t, err)
		assert.Equal(t, string(expectedKustomizationYaml), string(kustomizationYaml), "manifest")
		return nil
	}
	var writer bytes.Buffer
	err := Transform(&writer, kustomization)
	require.NoError(t, err, "Transform()")
	require.NotNil(t, opt, "render options")

	// error handling
	render = func(o kustomize.RenderOptions) error {
		return errors.New("errormock")
	}
	err = Transform(&bytes.Buffer{}, kustomization)
	require.Error(t, err, "Transform() should pass through render() error")

	render = func(o kustomize.RenderOptions) error { return nil }
	kustomization = func(dir string) (*TransformOptions, error) {
		return nil, errors.New("mock options error")
	}
	err = Transform(&bytes.Buffer{}, kustomization)
	require.Error(t, err, "Transform() should pass through options generator error")

	render = func(o kustomize.RenderOptions) error { return nil }
	kustomization = func(dir string) (*TransformOptions, error) {
		return &TransformOptions{}, nil
	}
	err = Transform(&bytes.Buffer{}, kustomization)
	require.Error(t, err, "Transform() should reject empty kustomization config")
}
