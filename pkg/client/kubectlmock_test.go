package client

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func init() {
	if os.Getenv("K8SCLIENTTEST_CALLS") != "" {
		if err := mockKubectl(); err != nil {
			fmt.Fprintf(os.Stderr, "mock kubectl: %s\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}

// mock kubectl calls
func mockKubectl() (err error) {
	mockedError := os.Getenv("K8SCLIENTTEST_ERROR")
	if mockedError != "" {
		return fmt.Errorf("%s", mockedError)
	}
	argStr := strings.Join(os.Args[1:], " ")
	kubectlCallFile := os.Getenv("K8SCLIENTTEST_CALLS")

	// Only allow expected mock calls provided with env
	allowed := false
	allowedCalls := strings.Split(os.Getenv("K8SCLIENTTEST"), "\n")
	for _, allowedCall := range allowedCalls {
		if allowedCall == argStr {
			allowed = true
			break
		}
	}
	if !allowed {
		err = fmt.Errorf("invalid mock call:\n  %s\nexpected one of:\n  %s", argStr, strings.Join(allowedCalls, "\n  "))
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

	if mockOutput := os.Getenv("K8SCLIENTTEST_OUT"); mockOutput != "" {
		_, err = fmt.Fprint(os.Stdout, mockOutput)
	}
	return
}

func assertKubectlCalls(t *testing.T, expectedCalls []string, mockOutput []byte, testee func(K8sClient) error) {
	tmpBin, err := ioutil.TempDir("", "k8spkg-test-")
	require.NoError(t, err)
	kubectlCallFile := filepath.Join(tmpBin, filepath.Base(tmpBin)+"-calls")
	defer os.RemoveAll(tmpBin)
	err = os.Symlink("/proc/self/exe", filepath.Join(tmpBin, "kubectl"))
	require.NoError(t, err)
	err = os.Setenv("PATH", tmpBin+string(filepath.ListSeparator)+os.Getenv("PATH"))
	require.NoError(t, err)

	for _, kubeconfig := range []string{"", "mykubeconf.yaml"} {
		if kubeconfig != "" {
			for i := range expectedCalls {
				expectedCalls[i] += " --kubeconfig " + kubeconfig
			}
		}

		fmt.Printf("TEST %s...\n", expectedCalls[0])
		ioutil.WriteFile(kubectlCallFile, []byte{}, 0644)
		err = os.Setenv("K8SCLIENTTEST_OUT", string(mockOutput))
		require.NoError(t, err)
		err = os.Setenv("K8SCLIENTTEST", strings.Join(expectedCalls, "\n"))
		require.NoError(t, err)
		err = os.Setenv("K8SCLIENTTEST_CALLS", kubectlCallFile)
		require.NoError(t, err)
		defer func() { os.Unsetenv("K8SCLIENTTEST_CALLS") }()
		os.Unsetenv("K8SCLIENTTEST_ERROR")

		// success case
		c := NewK8sClient(kubeconfig)
		err = testee(c)
		require.NoError(t, err)
		actualCalls, err := trackedKubectlCalls(kubectlCallFile)
		require.NoError(t, err, "tracked kubectl calls")
		expectedCallMap := map[string]bool{}
		for _, call := range expectedCalls {
			expectedCallMap[call] = true
		}
		require.Equal(t, expectedCallMap, actualCalls, "kubectl call")

		// error case
		mockedError := "mocked kubectl error"
		err = os.Setenv("K8SCLIENTTEST_ERROR", mockedError)
		require.NoError(t, err)
		err = testee(c)
		os.Unsetenv("K8SCLIENTTEST_ERROR")
		require.Error(t, err, "should pass through kubectl error")
		require.Contains(t, err.Error(), mockedError, "error should contain message passed through from kubectl")

		// unexpected output case
		if len(mockOutput) > 0 {
			err = os.Setenv("K8SCLIENTTEST_OUT", "{broken json}}")
			require.NoError(t, err)
			err = testee(c)
			require.Error(t, err, "should pass through kubectl output unmarshal error")
		}
	}
}

func trackedKubectlCalls(kubectlCallFile string) (calls map[string]bool, err error) {
	f, err := os.Open(kubectlCallFile)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	calls = map[string]bool{}
	for scanner.Scan() {
		calls[scanner.Text()] = true
	}
	return
}
