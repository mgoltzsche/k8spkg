// +build kubectl_kustomize

package kustomize

import (
	"fmt"
	"os"
)

func init() {
	if os.Args[0] == "kubectl" && len(os.Args) == 3 && os.Args[1] == "kustomize" {
		// mock `kubectl kustomize`
		if os.Args[2] == "test/kustomizedpkg" {
			fmt.Println(`---
apiVersion: certmanager.k8s.io/v1alpha1
kind: Certificate
metadata:
  name: mycert
  namespace: mynamespace
  labels:
    app.kubernetes.io/part-of: kustomizedpkg
spec:
  duration: 23000h
---
apiVersion: v1
kind: Deployment
metadata:
  name: mydeployment
  namespace: mynamespace
  labels:
    app.kubernetes.io/part-of: kustomizedpkg
spec:
  duration: 23000h`)
			os.Exit(0)
		} else {
			fmt.Fprintf(os.Stderr, "KUBECTLMOCK: input file %s not mapped\n", os.Args[2])
		}
		fmt.Fprintf(os.Stderr, "KUBECTLMOCK: unexpected kustomize call: %+v\n", os.Args)
		os.Exit(1)
	}
}
