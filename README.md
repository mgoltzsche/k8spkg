k8spkg [![Build Status](https://travis-ci.org/mgoltzsche/k8spkg.svg?branch=master)](https://travis-ci.org/mgoltzsche/k8spkg)
=

A small wrapper CLI around kubectl to transform, deploy, undeploy and retrieve
Kubernetes manifests as a deployment unit waiting for the respective task
to be completed.
k8spkg accepts manifest files and
[kustomizations](https://github.com/kubernetes-sigs/kustomize)
like kubectl.

## Features

- Rollout a manifest waiting for the contained APIServices and Deployments to become available.
- Add common label to a manifest's API objects to identify them as a deployment unit (using [kustomize](https://github.com/kubernetes-sigs/kustomize)).
- Delete API objects by package name or source waiting for them to be deleted.
- List installed deployment units (packages)
- [kustomization](https://github.com/kubernetes-sigs/kustomize) support

## Requirements

- [Git](https://git-scm.com/) 2.3
- [Kubernetes](https://github.com/kubernetes/kubernetes) 1.13.5

## Usage

| Usage | Description |
|-------|-------------|
| `k8spkg manifest {-f SRC\|-k SRC} [--name <PKG>] [--namespace <NS>] [--timeout <DURATION>]` | Prints a merged and labeled manifest |
| `k8spkg apply {-f SRC\|-k SRC} [--name <PKG>] [--namespace <NS>] [--timeout <DURATION>] [--prune]` | Installs or updates the provided source as package and waits for the rollout to succeed |
| `k8spkg delete {-f SRC\|-k SRC\|PKG} [--namespace <NS>] [--timeout <DURATION>]` | Deletes the identified objects from the cluster and awaits their deletion. A package's API objects in other namespaces that are referred to (label) within global API objects are deleted as well. |
| `k8spkg list [--namespace <NS>] [--timeout <DURATION>]` | Lists the installed packages that are visible within the namespace. Other namespaces are not queried. However packages of global API objects and their referenced (label) namespaces are listed as well. |

## Examples

Print labeled manifest of the deployment unit `cert-manager`:
```
$ k8spkg manifest --name cert-manager -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.7/deploy/manifests/cert-manager.yaml
<YAML output>
```

Label and deploy `cert-manager`:
```
k8spkg apply --name cert-manager -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.7/deploy/manifests/cert-manager.yaml &&
k8spkg apply --name cert-manager-ca-issuer -f ca-issuer.yaml
```

List the installed packages:
```
$ k8spkg list
PACKAGE                   NAMESPACES
cert-manager              cert-manager,kube-system
```

List the installed packages wtthin `cert-manager` namespace:
```
$ k8spkg list
PACKAGE                   NAMESPACES
cert-manager              cert-manager,kube-system
cert-manager-ca-issuer    cert-manager
```

Label and deploy a kustomize package:
```
k8spkg apply --name hello-world -k github.com/kubernetes-sigs/kustomize//examples/helloWorld?ref=v2.1.0
```

Delete a previously installed package:
```
k8spkg delete cert-manager-ca-issuer
```

## Build

Build k8spkg using Docker:
```
make
```
