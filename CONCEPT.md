# Concept

## Motivation

None of kubectl, kustomize or helm allows to describe and install all k8s
resources with a single file and command.
Most of them have a [built-in resource order](https://github.com/kubernetes-sigs/kustomize/issues/821)
that is suitable for application of single packages but does not respect
crd's implicit orders, lack crd support and do not wait for required resources
to become available.
(For instance installing cert-manager and a custom issuer requires the caller
to wait for cert-manager to become available before applying the issuer.)
Waiting for all resources of a given manifest yaml to become available
in a generic way is currently not possible using the `kubectl` CLI only
resulting in dirty shell scripts.  

Both helm and kustomize have proven well when it comes to reusability:
helm's packages/community and kustomize' means to modify anything (helm's shortcoming).  
However helm does not support custom resource definitions, manages the state of
installed packages within an own configmap potentially inconsistent with the
actual cluster state and does operations using tiller,
the giant sudo service.  
kustomize is not expected to support whole cluster installations since
it is just a yaml renderer and not aware of the cluster state.


`kubectl` could support the requirements mentioned above but it does not (yet).
One could argue that `kubectl` is a lower-level tool and as such shouldn't
support everything but provide basic granular functionality only on which
higher-level tooling can build. However this granular functionality should be
enhanced but this will take some time and experience that may be provided with
this project.

## Package

A declaration of _sources_ and _dependencies_
identified by a unique _package ID_ (maybe its URL as in Go?).

* Sources: A collection of k8s manifests and/or kustomize project(s) merged into a single yaml that can be applied using `kubectl apply -f`.
* TBD: variables (to be passed through to kustomize (+helm?))
* TBD: profiles? (to declare additional/conditional sources & default var values)

### Source resolution

Source resolution happens in the following order:
* resolve remote source to file using go-getter
* if source is file: include as manifest file
* if source is directory containing `kustomization.yaml`: include rendered kustomization output
* if source is directory: include all `*.yaml` files as manifests

Sources can only refer to files within the package/descriptor's directory/path (scope/security).

### Helm support / YAML Generators

TBD: helm support using [kubecrt](https://github.com/blendle/kubecrt) or [helm-convert](https://github.com/ContainerSolutions/helm-convert) or by supporting dynamic extensions (project-specific?).

## Package installation

Deploy k8s components as package reliably...
* waiting for them to become available
* allowing to determine the actual package state within the cluster using labels
* deleting old package version's resources

With `kubectl apply -f - --prune -l` the k8s CLI allows to delete resources by
label that do not appear within the input yaml but it does not (yet) support
injecting the corresponding labels into the input yaml to mark them as parts of
a prunable package.  
The k8s CLI also supports waiting for resources to become available but not in
a generic way as the `condition` value of the `kubectl wait --for` command
option varies depending on the resource type. Currently one needs to filter
everything but `ApiService`s and `Deployment`s from the input yaml and wait
`--for condition=available`.

- Add a label to all objects within input yaml to identify the package state within the k8s cluster.
- Run `kubectl apply -f - --prune --d -l LABELQUERY` to install/update a package.
- Run `kubectl wait --for condition=available [-n NAMESPACE] {apiservice|deploy}/NAME`

Optionally all dependencies should be resolved and installed

## Deletion

- Delete by common label: `kubectl delete all --all-namespaces=true -l LABELQUERY`
- Wait until deleted: `kubectl wait --for delete`

## TL;DR: Thoughts about dependency management

This is a discussion about how dependent packages could be managed

Dependencies could be declared as:
* [go-getter](https://github.com/hashicorp/go-getter) URLs (local file access is only allowed within the package descriptor's directory)
* package IDs (resolved dynamically)

Cyclic dependencies are not allowed
Duplicate package IDs among a set of modules are not allowed -
or should they override each other in the order they are imported in?!
PROBLEM: Could happen accidentally and should be avoided.  
PRO: Only a required dependencies need to be downloaded - not the whole repository  

Other approach:
* repositories: directory with a descriptor and packages in form of subdirectories with each a package descriptor
* package: a directory within a repository, package named like the directory implicitly
* dependencies are declared as a list of package names that are resolved using the declared repositories in the order in which they occur
* repository overlays: an optional list of URLs per repository to other repositories whose packages are shadowed in the order in which they occur during recursive package lookups
PROBLEM: Adding a package to an upper repository could shadow an actively used package with the same name from a lower repository - this is error-prone and has serious security implications as well!  

Solution approach:
* add repository namespace to package names (not allowing implicit inheritance), disallowing duplicate repository
* _fully qualified name_ (FQN; namespace prefix?) should be used to identify packages (CLI + dependencies) to make sure that the right package is picked -  
* conflicts can still happen when referring to multiple versions or the same package ID within different namespaces
* -> a conflict that arose among dependencies of a package can be resolved by specifying a package with the conflicting ID in a higher level of the dependency tree
* -> fully qualified name should equal package location to make sure package names are unique (like golang imports) and make lookups fast
* reduce amount of yamls to edit (manually maintainable) in git repo but stay flexible? -> no separate repository descriptor but refer to dependencies by FQN and version. Usually only one main dependency of a repository will need to be referenced any way, is it?  
     PROBLEM: the version declaration would be repetitive (bad for manual maintainability - DRY)
* -> Alternatively, to have the repo version in one place and simplify dependency lookups, a separate repository descriptor (as described above, not as overlay but map) could be used to declare repository URLs with versions and resolve a corresponding package dependency version using these.
* -> Namespace prefix should be used to be able to refer to relative paths within repository descriptor (which would look wrong within package descriptor):  
     Hence repositories must be declared as a map (of prefix and versioned url) and a package dependency as: `[nsprefix:]id`  
* -> Under which name to store the package within the cluster uniquely? Prefix is rather local, local repository URL may change as well.  
     Repository needs an ID!  
     Should the ID be decoupled from the location?  
     Contra: ID clashes within the cluster are possible  
     Pro: CD environments change and the user may want to replace a package installation from one URL with one from another URL
* -> Repository ID could be used as package ID prefix used both within the cluster and dependency declarations.  
     Contra: all repositories would need to be loaded to know all IDs.  
     Pro: all repository descriptors (only!) must always be loaded anyway to make sure there are no duplicate IDs.  
     Contra: when repository identifier does not equal its location it's unclear from which URL a package dependency is coming from.
* -> Use local namespace prefix per repository URL!  
     Should the prefix equal the repository ID?  
     Pro: Consistency check: otherwise, changing a repository ID in a dependency would result in conflicting, duplicate installations
     Contra: Repetitive, more effort when renaming/swapping a repo. (referring to multiple versions of the same repository would not be possible - though usually you want to avoid that)


Solution:
A _repository_ descriptor declares a fully qualified identifier (FQN) that serves as namespace. The FQN _should_ equal its location (go-getter).
Repository imports can be declared as a map of namespace prefixes to URLs.  
A _package_ declares a name. Its ID is the name prefixed with its repository's ID.
Dependencies are declared as a list of package names prefixed with their repository's prefix. Packages within the same repository can be referred to by the repository-local name without prefix as well.  

TBD: Override package dependency

PROBLEM: For consistency/security: Repository ID must equal the repository URL (go-getter) in case of remote packages.
Otherwise changing a dependent repository's ID so that it overlays an installed package could delete the installed package.
PROBLEM: How to deal with parameters? Each package has a configuration of its own and may require different dependencies depending on the environment it should be installed in.

Solution: Don't support dependencies but let the user provide precisely the packages she likes to install and their order.
Still: It would be great to support local dependencies or rather multi-module packages in order to install cert-manager and some issuers afterwards
