package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgoltzsche/k8spkg/pkg/k8spkg"
	"github.com/mgoltzsche/k8spkg/pkg/model"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	if os.Getenv("K8SPKGTEST_CALLS") != "" {
		if err := mockKubectl(); err != nil {
			fmt.Fprintf(os.Stderr, "mock kubectl: %s\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}

// mock kubectl calls
func mockKubectl() (err error) {
	argStr := strings.Join(os.Args[1:], " ")

	// track kubectl call
	kubectlCallFile := os.Getenv("K8SPKGTEST_CALLS")
	f, err := os.OpenFile(kubectlCallFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	stdinLen := 0
	out := ""
	strippedArgs := os.Args[1:]
	if strippedArgs[0] == "--kubeconfig" {
		strippedArgs = strippedArgs[2:]
	}
	switch strippedArgs[0] {
	case "api-resources":
		out = `NAME                              SHORTNAMES   APIGROUP                       NAMESPACED   KIND
configmaps                        cm                                          true         ConfigMap
namespaces                        ns                                          false        Namespace
deployments                       deploy       extensions                     true         Deployment
customresourcedefinitions         crd,crds     apiextensions.k8s.io           false        CustomResourceDefinition`
	case "get":
		if strippedArgs[1] == "event" {
			out = `{
				"type": "Warning",
				"reason": "some reason",
				"message": "some message",
				"involvedObject": {
					"uid": "b99471c0-96d6-11e9-bafd-0242a54f69f8"
				}
			}`
		} else {
			var b []byte
			b, err = ioutil.ReadFile("../model/test/status/k8sobjectlist-status.yaml")
			out = string(b)
		}
	case "apply":
		var b []byte
		b, err = ioutil.ReadAll(os.Stdin)
		if err == nil {
			stdinLen = len(b)
		}
		b, err = ioutil.ReadFile("../model/test/k8sobjectlist.yaml")
		out = string(b)
	}
	fmt.Fprintf(f, "%d %s\n", stdinLen, argStr)
	fmt.Println(out)
	return
}

func assertKubectlVerbsUsed(t *testing.T, args, expectedVerbs []string, callMap map[string]string) {
	_, actualCalls, err := testRun(t, args)
	require.NoError(t, err)
	cmdMap := map[string]bool{}
	for _, cmd := range expectedVerbs {
		cmdMap[cmd] = false
	}
	actualCmds := []string{}
	for _, call := range actualCalls {
		cmdSegs := strings.Split(call, " ")
		verb := cmdSegs[1]
		if verb == "--kubeconfig" {
			verb = cmdSegs[3]
		}
		used, isExpected := cmdMap[verb]
		require.True(t, isExpected, "unexpected kubectl cmd %q used by %+v:\n  %s", verb, args, call)
		cmdMap[verb] = true
		if !used {
			actualCmds = append(actualCmds, verb)
		}
	}
	require.Equal(t, expectedVerbs, actualCmds, "used commands of %+v", args)

	// Check for unused options - different calls that result in same kubectl calls
	callsStr := strings.Join(actualCalls, "\n")
	argsStr := strings.Join(args, " ")
	prevTesteeCall, duplicateCallSet := callMap[callsStr]
	require.False(t, duplicateCallSet, "calls resulted in same kubectl calls:\n  %s\n  %s\n\nkubectl calls:\n  %s", prevTesteeCall, argsStr, strings.ReplaceAll(callsStr, "\n", "\n  "))
	callMap[callsStr] = argsStr
}

func TestManifest(t *testing.T) {
	srv := httptest.NewServer(http.FileServer(http.Dir("..")))
	addr := srv.Listener.Addr().String()
	defer srv.Close()

	for _, c := range []struct {
		expectedPkgName string
		expectedCount   int
		args            []string
	}{
		{"somepkg", 8, []string{"manifest", "-f", "../model/test"}},
		{"withname", 8, []string{"manifest", "-f", "../model/test", "--name", "withname"}},
		{"kustomizedpkg", 2, []string{"manifest", "-k", "../model/test/kustomize"}},
		{"remoteFile", 2, []string{"manifest", "-f", "http://" + addr + "/model/test/manifestdir/some-cert.yaml", "--name", "remoteFile"}},
	} {
		out, _, err := testRun(t, c.args)
		require.NoError(t, err)
		obj, err := model.FromReader(bytes.NewReader(out))
		require.NoError(t, err, "FromReader(%s)", c.expectedPkgName)
		require.Equal(t, c.expectedCount, len(obj), "%s object count", c.expectedPkgName)
		pkgName := ""
		for _, o := range obj {
			if pkgName = o.Labels()[k8spkg.PKG_NAME_LABEL]; pkgName != c.expectedPkgName {
				break
			}
		}
		assert.Equal(t, c.expectedPkgName, pkgName, "package name")
	}
}

func TestCLI(t *testing.T) {
	kubectlCallSets := map[string]string{}
	applyKubectlVerbs := []string{"apply", "api-resources", "get", "rollout", "wait"}
	for _, c := range []struct {
		args                 []string
		expectedKubectlVerbs []string
	}{
		{[]string{"manifest", "somepkg"}, []string{"api-resources", "get"}},
		{[]string{"-d", "manifest", "somepkg", "-n", "myns"}, []string{"api-resources", "get"}},
		{[]string{"manifest", "somepkg", "-n", "myns", "--kubeconfig", "kubeconfig.yaml"}, []string{"api-resources", "get"}},
		{[]string{"apply", "-f", "../model/test"}, applyKubectlVerbs},
		{[]string{"apply", "-f", "../model/test", "--timeout=3s"}, applyKubectlVerbs},
		{[]string{"apply", "-f", "../model/test", "-n", "myns", "--name", "renamedpkg"}, applyKubectlVerbs},
		{[]string{"apply", "-f", "../model/test/manifestdir", "-n", "myns", "--name", "renamedpkg"}, applyKubectlVerbs},
		{[]string{"apply", "-f", "../model/test", "-n", "myns", "--name", "renamedpkg", "--kubeconfig", "kubeconfig.yaml"}, applyKubectlVerbs},
		{[]string{"apply", "-k", "../model/test/kustomize"}, applyKubectlVerbs},
		{[]string{"apply", "-k", "../model/test/kustomize", "--timeout=3s"}, applyKubectlVerbs},
		{[]string{"apply", "-k", "../model/test/kustomize", "-n", "myns", "--name", "renamedpkg"}, applyKubectlVerbs},
		{[]string{"apply", "-k", "../model/test/kustomize", "-n", "myns", "--prune"}, applyKubectlVerbs},
		{[]string{"apply", "-k", "../model/test/kustomize", "-n", "myns", "--name", "renamedpkg", "--kubeconfig", "kubeconfig.yaml"}, applyKubectlVerbs},
		{[]string{"delete", "-f", "../model/test"}, []string{"delete", "wait"}},
		{[]string{"delete", "-f", "../model/test", "--timeout=3s"}, []string{"delete", "wait"}},
		{[]string{"delete", "-f", "../model/test/manifestdir"}, []string{"delete", "wait"}},
		{[]string{"delete", "-f", "../model/test", "-n", "myns"}, []string{"delete", "wait"}},
		{[]string{"delete", "-f", "../model/test", "-n", "myns", "--kubeconfig", "kubeconfig.yaml"}, []string{"delete", "wait"}},
		{[]string{"delete", "-k", "../model/test/kustomize"}, []string{"delete", "wait"}},
		{[]string{"delete", "-k", "../model/test/kustomize", "--timeout=3s"}, []string{"delete", "wait"}},
		{[]string{"delete", "-k", "../model/test/kustomize", "-n", "myns"}, []string{"delete", "wait"}},
		{[]string{"delete", "-k", "../model/test/kustomize", "-n", "myns", "--kubeconfig", "kubeconfig.yaml"}, []string{"delete", "wait"}},
		{[]string{"delete", "somepkg"}, []string{"api-resources", "get", "delete", "wait"}},
		{[]string{"delete", "somepkg", "--timeout=3s"}, []string{"api-resources", "get", "delete", "wait"}},
		{[]string{"delete", "somepkg", "-n", "myns"}, []string{"api-resources", "get", "delete", "wait"}},
		{[]string{"delete", "somepkg", "-n", "myns", "--kubeconfig", "kubeconfig.yaml"}, []string{"api-resources", "get", "delete", "wait"}},
		{[]string{"list"}, []string{"api-resources", "get"}},
		{[]string{"list", "-n", "myns"}, []string{"api-resources", "get"}},
		{[]string{"list", "-n", "myns", "--kubeconfig", "kubeconfig.yaml"}, []string{"api-resources", "get"}},
		{[]string{"list", "--all-namespaces"}, []string{"api-resources", "get"}},
	} {
		assertKubectlVerbsUsed(t, c.args, c.expectedKubectlVerbs, kubectlCallSets)
	}
}

func TestCLIErrorHandling(t *testing.T) {
	for _, args := range [][]string{
		{"manifest"},
		{"manifest", "-n", "myns"},
		{"apply"},
		{"apply", "../model/test"},
		{"apply", "../model/test", "-n", "myns"},
		{"apply", "../model/test", "--name", "renamedpkg"},
		{"apply", "../model/test", "-n", "myns", "--name", "renamedpkg"},
		{"delete"},
		{"list", "--all-namespaces", "-n", "myns"},
	} {
		_, _, err := testRun(t, args)
		require.Error(t, err, "%+v", args)
	}
}

func testRun(t *testing.T, args []string) (b []byte, actualCalls []string, err error) {
	// reset state
	kubeconfigFile = ""
	sourceKustomize = ""
	sourceFile = ""
	namespace = ""
	pkgName = ""
	allNamespaces = false
	prune = false

	// create temp PATH dir to make kubectl cmd resolve to /proc/self/exe
	tmpBin, err := ioutil.TempDir("", "k8spkg-test-bin-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpBin)
	err = os.Symlink("/proc/self/exe", filepath.Join(tmpBin, "kubectl"))
	require.NoError(t, err)
	err = os.Setenv("PATH", tmpBin+string(filepath.ListSeparator)+os.Getenv("PATH"))
	require.NoError(t, err)
	kubectlCallFile := filepath.Join(tmpBin, "tracked-kubectl-calls")
	err = os.Setenv("K8SPKGTEST_CALLS", kubectlCallFile)
	require.NoError(t, err)
	defer os.Unsetenv("K8SPKGTEST_CALLS")

	os.Args = append([]string{"k8spkg"}, args...)
	stdout := os.Stdout
	f, err := ioutil.TempFile("", "k8spkg-test-stdout-")
	require.NoError(t, err)
	fileName := f.Name()
	os.Stdout = f
	defer func() {
		os.Stdout = stdout
		f.Close()
		os.Remove(fileName)
	}()
	err = rootCmd.Execute()
	if err != nil {
		err = errors.Wrapf(err, "%+v", args)
	}
	f.Close()
	if err == nil {
		var e error
		b, e = ioutil.ReadFile(fileName)
		if e != nil {
			panic(e)
		}
	}
	actualCalls, e := trackedKubectlCalls(kubectlCallFile)
	require.NoError(t, e, "tracked kubectl calls of %+v", args)
	return
}

func trackedKubectlCalls(kubectlCallFile string) (calls []string, err error) {
	f, err := os.Open(kubectlCallFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		calls = append(calls, scanner.Text())
	}
	return
}
