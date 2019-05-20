# k8src

An experimental command line tool to manage Kubernetes manifests and
overcome current kubectl limitations.

## Goals

- Wait for required ApiServices (and Deployments) to become available before applying dependent objects.
- Manage kustomize as well as raw manifests with predefined order (kustomize defines its own order, see [cert-manager issue](https://github.com/kubernetes-sigs/kustomize/issues/821))
- Inject label into manifests to support removal by label and `kubectl apply --prune -l`