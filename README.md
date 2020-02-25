# test-environment-manager

> Manage test environments for pull requests

## Getting started

Make sure you have [GO](https://golang.org/doc/install) installed and
configured your [GOPATH](https://github.com/golang/go/wiki/SettingGOPATH).

```
$ git clone git@github.com:kolonialno/test-environment-manager.git $GOPATH/src/github.com/kolonialno/test-environment-manager
$ cd $GOPATH/src/github.com/kolonialno/test-environment-manager
$ make vendor # Install dependencies
```

## Development

### Packages

The test-environment-manager uses the operator-pattern to watch
test environments inside the Kubernetes cluster. The controller
is responsible for the following tasks:

- Answer GitHub webhooks and update status checks
- Build docker container for each environment
- Create a separate environment for each pull-request
- Provide terminal access to the environment
- Remove stale builds
- Orchestrate creation of databases, including data preparation

To do this, the test-environment-manager consist of the following packages:

- apis: Kubernetes api definitions for custom resources
- builder: Background worker orchestrating docker builds
- cleanup: Background worker used to detect old environments
- controller: Custom Kubernetes controller responsible for cluster resource management
- databaseprovisioner: Provision postgres databases for the test environments
- debug: Debug server used to expose application metrics
- github: Interface for interacting with GitHub
- internal: Internal utils
- k8s: Kubernetes api client
- status: Serves a statuspage before the environment is ready for traffic and a web-based terminal
- webhook: GitHub webhook server

### Running tests

```
$ make test
```

### Building the program

```
$ make build
```

### Push docker image to remote repository

```
$ make push
```

### Generate new CRDs

```
kubebuilder create api --group testenvironment --version v1alpha1 --kind Resource
```

### Libraries

- docker/docker/client - Docker client
- google/go-github - GitHub client
- k8s.io/apimachinery - Kubernetes API definitions
- sigs.k8s.io/controller-runtime - Kubernetes operator framework
