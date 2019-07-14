k8spkg [![Build Status](https://travis-ci.org/mgoltzsche/k8spkg.svg?branch=master)](https://travis-ci.org/mgoltzsche/k8spkg)
=

A wrapper around kubectl to transform, deploy, undeploy and retrieve
[Kubernetes](https://github.com/kubernetes/kubernetes) manifests as
a deployment unit waiting for the respective task to be completed.
k8spkg accepts manifest files and
[kustomizations](https://github.com/kubernetes-sigs/kustomize)
like kubectl.

## Features

- Maintain a group of Kubernetes API objects generically as "package" using [labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/) (`app.kubernetes.io/part-of`, `k8spkg.mgoltzsche.github.com/namespaces`).
- Add common labels to a manifest's API objects (using [kustomize](https://github.com/kubernetes-sigs/kustomize)).
- Rollout a manifest/package as single deployment unit: Wait for the contained Deployments, Services and APIServices to become available.
- List installed packages: Packages are visible within their API objects' namespace(s) only as long as they don't have global API objects as well.
- Delete API objects by package name or source and wait until they are deleted.
- [kustomization](https://github.com/kubernetes-sigs/kustomize) source support.

## Requirements

- [Git](https://git-scm.com/) 2.3 (optional)
- [Kubernetes](https://github.com/kubernetes/kubernetes) 1.11

## Usage

| Command | Description |
|-------|-------------|
| `manifest {-f SRC\|-k SRC} [--name <PKG>] [--namespace <NS>] [--timeout <DURATION>]` | Prints a merged and labeled manifest |
| `apply {-f SRC\|-k SRC} [--name <PKG>] [--namespace <NS>] [--timeout <DURATION>] [--prune]` | Installs or updates the provided source as package and waits for the rollout to succeed. `--prune` deletes all API objects labeled with the package name that do not appear within the source from the cluster - should be used carefully. |
| `delete {-f SRC\|-k SRC\|PKG} [--namespace <NS>] [--timeout <DURATION>]` | Deletes the identified objects from the cluster and awaits their deletion. A package's API objects in other namespaces that are referred to (label) within global API objects are deleted as well. |
| `list [--all-namespaces\|--namespace <NS>] [--timeout <DURATION>]` | Lists the installed packages that are visible within the namespace. Other namespaces are not queried as long as `--all-namespaces` is not enabled. However packages of global API objects and their referenced (label) namespaces are listed as well. |

### Examples

Print labeled manifest of the deployment unit `cert-manager`:
```
$ k8spkg manifest --name cert-manager -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.7/deploy/manifests/cert-manager.yaml
<YAML output>
```

Label and deploy `cert-manager` and a namespaced issuer afterwards:
```
k8spkg apply --name cert-manager -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.7/deploy/manifests/cert-manager.yaml &&
k8spkg apply --name cert-manager-ca-issuer -f ca-issuer.yaml
```
_Please note that this does not (yet?!) work with `kubectl apply` since it does not wait for API objects to be ready (cert-manager's APIService must accept the Issuer) and there is no option or other generic `kubectl` command to wait for such a state based on a given manifest. Fortunately `kubectl rollout` and `kubectl wait` serve this purpose but require object names and type-dependent options which k8spkg provides._  

List the installed packages from within the `default` namespace:
```
$ k8spkg list
PACKAGE                   NAMESPACES
cert-manager              cert-manager,kube-system
```

List the installed packages wtthin `cert-manager` namespace:
```
$ k8spkg list -n cert-manager
PACKAGE                   NAMESPACES
cert-manager              cert-manager,kube-system
cert-manager-ca-issuer    cert-manager
```

Label and deploy a kustomize package:
```
k8spkg apply --name hello-world -k github.com/kubernetes-sigs/kustomize//examples/helloWorld?ref=v2.1.0
```

Install or update a package in another namespace:
```
k8spkg apply -n mynamespace --name hello-world --prune -k github.com/kubernetes-sigs/kustomize//examples/helloWorld?ref=v2.1.0
```

Delete a previously installed package:
```
k8spkg delete -n cert-manager cert-manager-ca-issuer
```

## Install

[Download](https://github.com/mgoltzsche/k8spkg/releases/latest/download/k8spkg) and install the latest k8spkg release (static linux amd64):
```
curl -L https://github.com/mgoltzsche/k8spkg/releases/latest/download/k8spkg > k8spkg
chmod +x k8spkg
sudo mv k8spkg /usr/local/bin/k8spkg
```

## Build

Install/update with Go:
```
go get -u github.com/mgoltzsche/k8spkg
```
or run a dockerized k8spkg build:
```
git clone https://github.com/mgoltzsche/k8spkg
cd k8spkg && make
sudo mv k8spkg /usr/local/bin/
```  

_The project can be opened in a containerized [LiteIDE](https://github.com/visualfc/liteide) using `make ide`._

## License

k8spkg is licensed under [Apache License 2.0](./LICENSE).
Some of the 3rd party modules in the `vendor` directory are licensed under different Open Source conditions (see `LICENSE` files).
