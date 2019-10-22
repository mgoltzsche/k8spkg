package cmd

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/mgoltzsche/k8spkg/pkg/client"
	"github.com/mgoltzsche/k8spkg/pkg/client/mock"
	"github.com/mgoltzsche/k8spkg/pkg/k8spkg"
	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		verb := cmdSegs[0]
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
	if len(expectedVerbs) > 0 {
		require.False(t, duplicateCallSet, "calls resulted in same kubectl calls:\n  %s\n  %s\n\nkubectl calls:\n  %s", prevTesteeCall, argsStr, strings.ReplaceAll(callsStr, "\n", "\n  "))
	}
	callMap[callsStr] = argsStr
}

func TestBuild(t *testing.T) {
	srv := httptest.NewServer(http.FileServer(http.Dir("..")))
	addr := srv.Listener.Addr().String()
	defer srv.Close()

	for _, c := range []struct {
		expectedPkgName string
		expectedCount   int
		args            []string
	}{
		{"somepkg", 9, []string{"build", "-f", "../resource/test"}},
		{"withname", 9, []string{"build", "-f", "../resource/test", "--name", "withname"}},
		{"kustomizedpkg", 2, []string{"build", "-k", "../resource/test/kustomize"}},
		{"remoteFile", 2, []string{"build", "-f", "http://" + addr + "/resource/test/manifestdir/some-cert.yaml", "--name", "remoteFile"}},
	} {
		out, _, err := testRun(t, c.args)
		require.NoError(t, err)
		obj, err := resource.FromReader(bytes.NewReader(out))
		require.NoError(t, err, "FromReader(%s)", c.expectedPkgName)
		require.Equal(t, c.expectedCount, len(obj), "%s object count\nobjects:\n%s", c.expectedPkgName, strings.Join(obj.Refs().Names(), "\n"))
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
	applyKubectlVerbs := []string{"apply", "watch"}
	awaitDeletion := "awaitdeletion"
	for _, c := range []struct {
		args                 []string
		expectedKubectlVerbs []string
	}{
		{[]string{"build", "-f", "../resource/test"}, []string{}},
		{[]string{"build", "-f", "../resource/test", "-n", "myns", "-d"}, []string{}},
		{[]string{"apply", "-f", "../resource/test", "--timeout=3s"}, applyKubectlVerbs},
		{[]string{"apply", "-f", "../resource/test", "-n", "myns", "--name", "renamedpkg"}, applyKubectlVerbs},
		{[]string{"apply", "-f", "../resource/test/manifestdir", "-n", "myns", "--name", "renamedpkg"}, applyKubectlVerbs},
		{[]string{"apply", "-k", "../resource/test/kustomize", "--timeout=3s"}, applyKubectlVerbs},
		{[]string{"apply", "-k", "../resource/test/kustomize", "-n", "myns", "--name", "renamedpkg"}, applyKubectlVerbs},
		{[]string{"apply", "-k", "../resource/test/kustomize", "-n", "myns", "--prune"}, applyKubectlVerbs},
		{[]string{"delete", "-f", "../resource/test", "--timeout=3s"}, []string{"delete", awaitDeletion}},
		{[]string{"delete", "-f", "../resource/test/manifestdir"}, []string{"delete", awaitDeletion}},
		{[]string{"delete", "-f", "../resource/test", "-n", "myns"}, []string{"delete", awaitDeletion}},
		{[]string{"delete", "-k", "../resource/test/kustomize", "--timeout=3s"}, []string{"delete", awaitDeletion}},
		{[]string{"delete", "-k", "../resource/test/kustomize", "-n", "myns"}, []string{"delete", awaitDeletion}},
		{[]string{"delete", "somepkg", "--timeout=3s"}, []string{"getresource", "delete", awaitDeletion}},
		{[]string{"delete", "somepkg", "-n", "myns"}, []string{"getresource", "delete", awaitDeletion}},
		{[]string{"list"}, []string{"get"}},
		{[]string{"list", "-n", "myns"}, []string{"get"}},
		//TODO:{[]string{"list", "--all-namespaces"}, []string{"get"}},
	} {
		assertKubectlVerbsUsed(t, c.args, c.expectedKubectlVerbs, kubectlCallSets)
	}
}

func TestCLIErrorHandling(t *testing.T) {
	for _, args := range [][]string{
		{"unsupported"},
		{"build", "-n", "myns"},
		{"apply"},
		{"apply", "../resource/test"},
		{"apply", "../resource/test", "-n", "myns"},
		{"apply", "../resource/test", "--name", "renamedpkg"},
		{"apply", "../resource/test", "-n", "myns", "--name", "renamedpkg"},
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
	prune = false
	clientMock := mock.NewClientMock()
	clientMock.MockResource = mock.MockResourceList("../k8spkg/app-example.yaml")[0]
	clientFactory = func(kubeconfigFile string) client.K8sClient {
		return clientMock
	}

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
	actualCalls = clientMock.Calls
	return
}
