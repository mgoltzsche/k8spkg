package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(t *testing.T) {
	fo, err := ioutil.TempFile("", "k8spkg-stdout-")
	require.NoError(t, err)
	fileName := fo.Name()
	stdout := os.Stdout
	os.Stdout = fo
	defer func() {
		os.Stdout = stdout
		fo.Close()
		os.Remove(fileName)
	}()
	os.Args = []string{"k8spkg", "help"}
	main()
	os.Stdout = stdout
	fo.Close()
	b, err := ioutil.ReadFile(fileName)
	require.NoError(t, err)
	out := string(b)
	found := true
	for _, cmd := range []string{"apply", "manifest", "delete", "list", "version", "help"} {
		if !assert.Contains(t, out, "\n  "+cmd+" ") {
			found = false
		}
	}
	if !found {
		fmt.Println("STDOUT:\n" + out)
	}
}
