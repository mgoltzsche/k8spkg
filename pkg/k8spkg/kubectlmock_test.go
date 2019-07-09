package k8spkg

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	// Only allow provided expected mock calls provided with env
	allowedCallsEnv := os.Getenv("K8SPKGTEST")
	var allowedCalls []string
	if allowedCallsEnv != "" {
		allowedCalls = strings.Split(allowedCallsEnv, "\n")
	}
	calls, e := trackedKubectlCalls(kubectlCallFile)
	if e != nil {
		return e
	}
	if len(allowedCalls) <= len(calls) {
		err = fmt.Errorf("invalid mock call: %s", argStr)
	} else {
		expectedCall := allowedCalls[len(calls)]
		if expectedCall != argStr {
			err = fmt.Errorf("invalid mock call!\nexpected: %s\nactual:   %s", expectedCall, argStr)
		}
	}

	// track kubectl call
	af, e := os.OpenFile(kubectlCallFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if e != nil {
		return e
	}
	defer af.Close()
	fmt.Fprintf(af, "%s\n", argStr)

	if err != nil {
		return
	}

	// Mock logic
	switch strings.Join(os.Args[1:], " ") {
	case kubectlGetCall:
		err = printFile("../model/test/k8sobjectlist.yaml")
		err = printFile("../model/test/contained-pod-rs.yaml")
	case kubectlGetCallNsEmpty:
		err = printFile("../model/test/k8sobjectlist.yaml")
		err = printFile("../model/test/contained-pod-rs.yaml")
	case kubectlApplyCall:
		var f *os.File
		if f, err = os.OpenFile(os.Getenv("K8SPKGTEST_STDIN"), os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer f.Close()
			_, err = io.Copy(f, os.Stdin)
		}
	case kubectlResTypeCall:
		fmt.Print(resTypeTable)
	}
	return
}

func printFile(file string) (err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	_, err = io.Copy(os.Stdout, f)
	return
}

func trackedKubectlCalls(kubectlCallFile string) (calls []string, err error) {
	f, err := os.Open(kubectlCallFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		} else {
			return
		}
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		calls = append(calls, scanner.Text())
	}
	return
}

func assertKubectlCalls(t *testing.T, expectedCalls []string, errorAfterCall int, testee func(stdinFile string)) {
	tmpBin, err := ioutil.TempDir("", "k8spkg-test-")
	require.NoError(t, err)
	kubectlCallFile := filepath.Join(tmpBin, filepath.Base(tmpBin)+"-calls")
	stdinFile := filepath.Join(tmpBin, filepath.Base(tmpBin)+"-stdin")
	defer os.RemoveAll(tmpBin)
	err = os.Symlink("/proc/self/exe", filepath.Join(tmpBin, "kubectl"))
	require.NoError(t, err)
	err = os.Setenv("PATH", tmpBin+string(filepath.ListSeparator)+os.Getenv("PATH"))
	require.NoError(t, err)
	err = os.Setenv("K8SPKGTEST", strings.Join(expectedCalls[:errorAfterCall], "\n"))
	require.NoError(t, err)
	err = os.Setenv("K8SPKGTEST_CALLS", kubectlCallFile)
	require.NoError(t, err)
	err = os.Setenv("K8SPKGTEST_STDIN", stdinFile)
	require.NoError(t, err)
	testee(stdinFile)
	actualCalls, err := trackedKubectlCalls(kubectlCallFile)
	require.NoError(t, err, "tracked kubectl calls")
	assert.Equal(t, expectedCalls, actualCalls, "kubectl calls")
}
