package k8spkg

/*
import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

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
	strippedArgs := os.Args[1:]
	if strippedArgs[0] == "--kubeconfig" {
		strippedArgs = strippedArgs[2:]
	}
	joinedArgs := strings.Join(strippedArgs, " ")
	kubectlCallFile := os.Getenv("K8SPKGTEST_CALLS")

	if joinedArgs != kubectlWatchEventsCall {
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
	}

	if err != nil {
		return
	}

	// Mock logic
	objInOtherNs := `---
apiVersion: someapi/v1
kind: SomeKind
metadata:
  name: someobj
  namespace: othernamespace
  labels:
    app.kubernetes.io/part-of: pkg-othernamespace
`
	switch joinedArgs {
	case kubectlGetCall:
		err = printFile("../resource/test/k8sobjectlist.yaml")
		err = printFile("../resource/test/contained-pod-rs.yaml")
	case kubectlGetCallNsEmpty:
		err = printFile("../resource/test/status/k8sobjectlist-status.yaml")
		err = printFile("../resource/test/contained-pod-rs.yaml")
	case kubectlGetCallNsCertManager:
		err = printFile("../resource/test/kustomize/mycert.yaml")
	case kubectlGetObjStatusCall:
		err = printFile("../resource/test/status/k8sobjectlist-status.yaml")
	case kubectlListCall:
		err = printFile("../resource/test/k8sobjectlist.yaml")
		err = printFile("../resource/test/contained-pod-rs.yaml")
		fmt.Println(objInOtherNs)
	case kubectlListCallNsEmpty:
		err = printFile("../resource/test/k8sobjectlist.yaml")
		err = printFile("../resource/test/contained-pod-rs.yaml")
		fmt.Println(objInOtherNs)
	case kubectlListCallAllNamespaces:
		err = printFile("../resource/test/k8sobjectlist.yaml")
		err = printFile("../resource/test/contained-pod-rs.yaml")
		fmt.Println(objInOtherNs)
	case kubectlApplyCallPrune:
		b, err := ioutil.ReadAll(os.Stdin)
		if err == nil {
			if err = ioutil.WriteFile(os.Getenv("K8SPKGTEST_STDIN"), b, 0644); err == nil {
				err = printFile("../resource/test/k8sobjectlist.yaml")
			}
		}
	case kubectlResTypeCall:
		fmt.Print(resTypeTable)
	case kubectlWatchEventsCall:
		fmt.Println(`{
			"type": "Warning",
			"reason": "some reason",
			"message": "some message",
			"lastTimestamp": "2019-08-21T00:00:00Z",
			"involvedObject": {
				"uid": "b99471c0-96d6-11e9-bafd-0242a54f69f8"
			}
		}`)
		fmt.Println(`{
			"type": "Warning",
			"reason": "another reason",
			"message": "another message",
			"lastTimestamp": "2019-08-21T00:00:01Z",
			"involvedObject": {
				"uid": "another-uid"
			}
		}`)
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, time.Duration(5*time.Second))
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigs
			cancel()
		}()
		select {
		case <-ctx.Done():
			if e := ctx.Err(); e != context.Canceled {
				err = e
			}
		}
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
	defer func() {
		os.Unsetenv("K8SPKGTEST_CALLS")
	}()
	err = os.Setenv("K8SPKGTEST_STDIN", stdinFile)
	require.NoError(t, err)
	testee(stdinFile)
	actualCalls, err := trackedKubectlCalls(kubectlCallFile)
	require.NoError(t, err, "tracked kubectl calls")
	require.Equal(t, expectedCalls, actualCalls, "kubectl calls")
}
*/
