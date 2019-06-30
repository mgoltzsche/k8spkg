# k8spkg

An experimental command line tool to manage Kubernetes manifests and
overcome current kubectl limitations.

## Features

- Wait for required ApiServices and Deployments to become available before applying dependent objects.  
  _(for instance to deploy cert-manager and custom issuers/certs)_
- Add common package label to API objecfs.  
  _(to ease deployment state inspection and support `kubectl apply -f - --prune -l app.kubernetes.io/part-of=<PKG>`)_
- Delete API objects by package name.  
  _(`kubectl delete -l app.kubernetes.io/part-of=<PKG>`)_

## Requirements

- [Git](https://git-scm.com/) 2.3
- [Kubernetes](https://github.com/kubernetes/kubernetes) 1.13.5

## Usage

| Usage | Description |
|-------|-------------|
| `k8spkg manifest {-f SRC\|-k SRC} [--name <PKG>] [--timeout <DURATION>]` | Prints a merged and labeled manifest |
| `k8spkg apply {-f SRC\|-k SRC} [--name <PKG>] [--timeout <DURATION>] [--prune]` | Installs or updates the provided source as package waiting for successful rollout |
| `k8spkg delete {-f SRC\|-k SRC\|PKG} [--timeout <DURATION>]` | Deletes the identified objects from the cluster |

## Examples

Print labeled manifest of `cert-manager`:
```
k8spkg manifest --name cert-manager -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.7/deploy/manifests/cert-manager.yaml
<YAML output>
```

Install `cert-manager`:
```
k8spkg apply --name cert-manager -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.7/deploy/manifests/cert-manager.yaml &&
k8spkg apply --name cert-manager-ca-issuer -f ca-issuer.yaml
```

Delete `cert-manager`:
```
k8spkg delete cert-manager
```

Deploy kustomize package:
```
k8spkg apply --name exampleapp -k github.com/kubernetes-sigs/kustomize//examples/helloWorld?ref=v2.1.0
```