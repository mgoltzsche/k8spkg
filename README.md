# k8spkg

An experimental command line tool to (un)deploy Kubernetes manifests and
overcome current kubectl limitations.

## Features

- Rollout a manifest waiting for the contained APIServices and Deployments to become available.
- Add common label to a manifest's API objects to identify them as package (using [kustomize](https://github.com/kubernetes-sigs/kustomize)).
- Delete API objects by package name waiting for them to be deleted.

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

Label and install `cert-manager`:
```
k8spkg apply --name cert-manager -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.7/deploy/manifests/cert-manager.yaml &&
k8spkg apply --name cert-manager-ca-issuer -f ca-issuer.yaml
```

Delete `cert-manager`:
```
k8spkg delete cert-manager
```

Label and deploy a kustomize package:
```
k8spkg apply --name exampleapp -k github.com/kubernetes-sigs/kustomize//examples/helloWorld?ref=v2.1.0
```