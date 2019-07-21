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
	kubectlCallFile := os.Getenv("K8SPKGTEST_CALLS")
	// track kubectl call
	f, err := os.OpenFile(kubectlCallFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\n", argStr)
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
		out = `
apiVersion: certmanager.k8s.io/v1alpha1
kind: Issuer
metadata:
    name: ca-issuer
    lamespace: cert-manager
    labels:
        app.kubernetes.io/part-of: somepkg`
	}
	fmt.Println(out)
	return
}

func assertKubectlCmdsUsed(t *testing.T, args, expectedCmds []string, callMap map[string]string) {
	tmpBin, err := ioutil.TempDir("", "k8spkg-test-")
	require.NoError(t, err)
	kubectlCallFile := filepath.Join(tmpBin, filepath.Base(tmpBin)+"-calls")
	defer os.RemoveAll(tmpBin)
	err = os.Symlink("/proc/self/exe", filepath.Join(tmpBin, "kubectl"))
	require.NoError(t, err)
	err = os.Setenv("PATH", tmpBin+string(filepath.ListSeparator)+os.Getenv("PATH"))
	require.NoError(t, err)
	err = os.Setenv("K8SPKGTEST_CALLS", kubectlCallFile)
	require.NoError(t, err)
	defer func() {
		os.Unsetenv("K8SPKGTEST_CALLS")
	}()
	testRun(t, args)
	cmdMap := map[string]bool{}
	for _, cmd := range expectedCmds {
		cmdMap[cmd] = false
	}
	actualCalls, err := trackedKubectlCalls(kubectlCallFile)
	require.NoError(t, err, "tracked kubectl calls of %+v", args)
	actualCmds := []string{}
	for _, call := range actualCalls {
		cmdSegs := strings.Split(call, " ")
		cmd := cmdSegs[0]
		if cmd == "--kubeconfig" {
			cmd = cmdSegs[2]
		}
		used, isExpected := cmdMap[cmd]
		require.True(t, isExpected, "unexpected kubectl call for %+v:\n  %s", args, call)
		cmdMap[cmd] = true
		if !used {
			actualCmds = append(actualCmds, cmd)
		}
	}
	callsStr := strings.Join(actualCalls, "\n")
	argsStr := strings.Join(args, " ")
	prevTesteeCall, duplicateCallSet := callMap[callsStr]
	require.False(t, duplicateCallSet, "calls resulted in same kubectl calls:\n  %s\n  %s\n\nkubectl calls:\n  %s", prevTesteeCall, argsStr, strings.ReplaceAll(callsStr, "\n", "\n  "))
	callMap[callsStr] = argsStr
	require.Equal(t, expectedCmds, actualCmds, "used commands of %+v", args)
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
		out := testRun(t, c.args)
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
	for _, c := range []struct {
		args                []string
		expectedKubectlCmds []string
	}{
		{[]string{"manifest", "somepkg"}, []string{"api-resources", "get"}},
		{[]string{"manifest", "somepkg", "-n", "myns"}, []string{"api-resources", "get"}},
		{[]string{"manifest", "somepkg", "-n", "myns", "--kubeconfig", "kubeconfig.yaml"}, []string{"api-resources", "get"}},
		{[]string{"apply", "-f", "../model/test"}, []string{"apply", "rollout", "wait"}},
		{[]string{"apply", "-f", "../model/test", "-n", "myns", "--name", "renamedpkg"}, []string{"apply", "rollout", "wait"}},
		{[]string{"apply", "-f", "../model/test", "-n", "myns", "--name", "renamedpkg", "--kubeconfig", "kubeconfig.yaml"}, []string{"apply", "rollout", "wait"}},
		{[]string{"apply", "-k", "../model/test/kustomize"}, []string{"apply", "rollout", "wait"}},
		{[]string{"apply", "-k", "../model/test/kustomize", "-n", "myns", "--name", "renamedpkg"}, []string{"apply", "rollout", "wait"}},
		{[]string{"apply", "-k", "../model/test/kustomize", "-n", "myns", "--prune"}, []string{"apply", "rollout", "wait"}},
		{[]string{"apply", "-k", "../model/test/kustomize", "-n", "myns", "--name", "renamedpkg", "--kubeconfig", "kubeconfig.yaml"}, []string{"apply", "rollout", "wait"}},
		{[]string{"delete", "-f", "../model/test"}, []string{"delete", "wait"}},
		{[]string{"delete", "-f", "../model/test", "-n", "myns"}, []string{"delete", "wait"}},
		{[]string{"delete", "-f", "../model/test", "-n", "myns", "--kubeconfig", "kubeconfig.yaml"}, []string{"delete", "wait"}},
		{[]string{"delete", "-k", "../model/test/kustomize"}, []string{"delete", "wait"}},
		{[]string{"delete", "-k", "../model/test/kustomize", "-n", "myns"}, []string{"delete", "wait"}},
		{[]string{"delete", "-k", "../model/test/kustomize", "-n", "myns", "--kubeconfig", "kubeconfig.yaml"}, []string{"delete", "wait"}},
		{[]string{"delete", "somepkg"}, []string{"api-resources", "get", "delete", "wait"}},
		{[]string{"delete", "somepkg", "-n", "myns"}, []string{"api-resources", "get", "delete", "wait"}},
		{[]string{"delete", "somepkg", "-n", "myns", "--kubeconfig", "kubeconfig.yaml"}, []string{"api-resources", "get", "delete", "wait"}},
		{[]string{"list"}, []string{"api-resources", "get"}},
		{[]string{"list", "-n", "myns"}, []string{"api-resources", "get"}},
		{[]string{"list", "-n", "myns", "--kubeconfig", "kubeconfig.yaml"}, []string{"api-resources", "get"}},
		{[]string{"list", "--all-namespaces"}, []string{"api-resources", "get"}},
	} {
		assertKubectlCmdsUsed(t, c.args, c.expectedKubectlCmds, kubectlCallSets)
	}
}

func testRun(t *testing.T, args []string) []byte {
	kubeconfigFile = ""
	sourceKustomize = ""
	sourceFile = ""
	namespace = ""
	pkgName = ""
	allNamespaces = false
	prune = false

	os.Args = append([]string{"k8spkg"}, args...)
	stdout := os.Stdout
	f, err := ioutil.TempFile("", "k8spkg-stdout-")
	require.NoError(t, err)
	fileName := f.Name()
	os.Stdout = f
	defer func() {
		os.Stdout = stdout
		f.Close()
		os.Remove(fileName)
	}()
	err = rootCmd.Execute()
	require.NoError(t, err, "%+v", args)
	f.Close()
	b, err := ioutil.ReadFile(fileName)
	require.NoError(t, err)
	return b
}
