package resource

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestManifestReader(t *testing.T) {
	srv := httptest.NewServer(http.FileServer(http.Dir(".")))
	addr := srv.Listener.Addr().String()
	defer srv.Close()

	baseDir, err := os.Getwd()
	require.NoError(t, err)

	for _, c := range []struct {
		source        string
		expectedCount int
		expectedNames []string
	}{
		{"test/manifestdir/some-cert.yaml", 2, []string{"somecert", "acert"}},
		{"test/manifestdir", 3, []string{"some-ca", "somecert", "acert"}},
		{"http://" + addr + "/test/manifestdir/some-cert.yaml", 2, []string{"somecert", "acert"}},
		{"http://" + addr + "/test/manifestdir/some-manifest", 1, []string{"some-object"}},
		//{"github.com/kubernetes-sigs/kustomize//examples/helloWorld?ref=v2.1.0", 4, []string{"the-map", "the-deployment", "the-service"}},
	} {
		ctx := context.Background()
		reader := ManifestReader(ctx, c.source, baseDir)
		b, err := ioutil.ReadAll(reader)
		require.NoError(t, err, "ManifestReader(%s)", c.source)
		manifest := string(b)

		// verify objects appear
		names := []string{}
		for _, name := range c.expectedNames {
			if strings.Contains(manifest, " name: "+name+"\n") {
				names = append(names, name)
			}
		}
		assert.Equal(t, c.expectedNames, names, "source %s", c.source)

		// verify valid yaml
		dec := yaml.NewDecoder(bytes.NewReader(b))
		o := map[string]interface{}{}
		count := 0
		for ; err == nil; err = dec.Decode(o) {
			if len(o) > 0 {
				count += 1
			}
			o = map[string]interface{}{}
		}
		require.True(t, err == io.EOF, "%s: yaml parser should yield EOF error but was %+v", c.source, err)
		assert.Equal(t, c.expectedCount, count, "%s object count", c.source)
	}
}

func TestManifestReaderError(t *testing.T) {
	ctx := context.Background()
	wd, err := os.Getwd()
	require.NoError(t, err)
	reader := ManifestReader(ctx, "some-none-existing-file", wd)
	_, err = ioutil.ReadAll(reader)
	require.Error(t, err)
}
