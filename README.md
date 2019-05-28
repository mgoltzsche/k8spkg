# k8spkg

An experimental command line tool to manage Kubernetes manifests and
overcome current kubectl limitations.

THIS PROJECT IS IN EARLY DEVELOPMENT STATE.

## Goals

- Wait for required ApiServices (and Deployments) to become available before applying dependent objects.
- Manage kustomize as well as raw manifests with predefined order (kustomize defines its own order, see [cert-manager issue](https://github.com/kubernetes-sigs/kustomize/issues/821))
- Inject label into manifests to support removal by label and `kubectl apply --prune -l`