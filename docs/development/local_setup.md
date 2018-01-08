# Preparing the Local Development Setup (Mac OS X)

Conceptionally, the Node Controller Manager is designed to run in a container within a Pod inside a Kubernetes cluster. For development purposes, you can run the Node Controller Manager as a Go process on your local machine. This process connects to your remote cluster to manage VMs for that cluster. That means that the Node Controller Manager runs outside a Kubernetes cluster which requires providing a [Kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/authenticate-across-clusters-kubeconfig/) in your local filesystem and point the Node Controller Manager to it when running it (see below). 

Although the following installation instructions are for Mac OS X, similar alternate commands could be found for any Linux distribution.

### Installing Golang environment
Install the latest version of Golang (at least `v1.8.3` is required) by using [Homebrew](https://brew.sh/):

```bash
$ brew install golang
```

Make sure to set your `$GOPATH` environment variable properly (conventionally, it points to `$HOME/go`).

For your convenience, you can add the `bin` directory of the `$GOPATH` to your `$PATH`: `PATH=$PATH:$GOPATH/bin`, but it is not necessarily required.

In order to perform linting on the Go source code, install [Golint](https://github.com/golang/lint):

```bash
$ go get -u github.com/golang/lint/golint
```

[Dep](https://github.com/golang/dep) is used for managing Golang package dependencies. Install it:
```bash
$ brew install dep
```

### Installing `Docker` (Optional)
In case you want to build Docker images for the Node Controller Manager you have to install Docker itself. We recommend using [Docker for Mac OS X](https://docs.docker.com/docker-for-mac/) which can be downloaded from [here](https://download.docker.com/mac/stable/Docker.dmg).

### Setup `Docker Hub` account (Optional)
Create a Docker hub account at [Docker Hub](https://hub.docker.com/) if you don't already have one.

## Local development

:warning: Before you start developing, please ensure to comply with the following requirements:

1. You have understood the [principles of Kubernetes](https://kubernetes.io/docs/concepts/), and its [components](https://kubernetes.io/docs/concepts/overview/components/), what their purpose is and how they interact with each other.
1. You have understood the [architecture of the Node Controller Manager](../design/node_controller_manager.md)

The development of the Node Controller Manager could happen by targetting any cluster. You basically need a Kubernetes cluster running on a set of machines. You just need the [Kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/authenticate-across-clusters-kubeconfig/) file with the required access permissions attached to it.

### Installing the Node Controller Manager locally
Clone the repository from GitHub into your `$GOPATH`.

```bash
$ mkdir -p $GOPATH/src/github.com/gardener
$ cd $GOPATH/src/github.com/gardener
$ git clone git@github.com:gardener/node-controller-manager.git
$ cd node-controller-manager
```

### Prepare the cluster

- Connect to the remote kubernetes cluster where you plan to deploy the Node Controller Manager using kubectl. Set the environment variable KUBECONFIG to the path of the yaml file containing your cluster info
- Now, create the required CRDs on the remote cluster using the following command,
```bash
$ kubectl apply -f kubernetes/crds.yaml
```

### Get started

Create a `dev` directory and copy the Kubeconfig of the kubernetes cluster used for development purposes to `dev/kubeconfig.yaml`.

- There is a rule dev in the `Makefile` which will automatically start the Node Controller Manager with development settings:

```bash
$ make dev
I1227 11:08:19.963638   55523 controllermanager.go:204] Starting shared informers
I1227 11:08:20.766085   55523 controller.go:247] Starting Node-controller-manager
```

The Node Controller Manager should now be ready to manage the VMs in your kubernetes cluster.

## Usage

To start using Node Controller Manager, follow the links given at [usage here](../README.md).