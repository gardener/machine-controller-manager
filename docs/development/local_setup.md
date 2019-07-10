# Preparing the Local Development Setup (Mac OS X)

<!-- TOC -->

- [Preparing the Local Development Setup (Mac OS X)](#preparing-the-local-development-setup-mac-os-x)
	- [Installing Golang environment](#installing-golang-environment)
	- [Installing `Docker` (Optional)](#installing-docker-optional)
	- [Setup `Docker Hub` account (Optional)](#setup-docker-hub-account-optional)
	- [Local development](#local-development)
		- [Installing the Machine Controller Manager locally](#installing-the-machine-controller-manager-locally)
	- [Prepare the cluster](#prepare-the-cluster)
	- [Getting started](#getting-started)
	- [Testing Machine Classes](#testing-machine-classes)
	- [Usage](#usage)

<!-- /TOC -->

Conceptionally, the Machine Controller Manager is designed to run in a container within a Pod inside a Kubernetes cluster. For development purposes, you can run the Machine Controller Manager as a Go process on your local machine. This process connects to your remote cluster to manage VMs for that cluster. That means that the Machine Controller Manager runs outside a Kubernetes cluster which requires providing a [Kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/authenticate-across-clusters-kubeconfig/) in your local filesystem and point the Machine Controller Manager to it when running it (see below).

Although the following installation instructions are for Mac OS X, similar alternate commands could be found for any Linux distribution.

## Installing Golang environment

Install the latest version of Golang (at least `v1.8.3` is required) by using [Homebrew](https://brew.sh/):

```bash
$ brew install golang
```

In order to perform linting on the Go source code, install [Golint](https://github.com/golang/lint):

```bash
$ go get -u golang.org/x/lint/golint
```

## Installing `Docker` (Optional)
In case you want to build Docker images for the Machine Controller Manager you have to install Docker itself. We recommend using [Docker for Mac OS X](https://docs.docker.com/docker-for-mac/) which can be downloaded from [here](https://download.docker.com/mac/stable/Docker.dmg).

## Setup `Docker Hub` account (Optional)
Create a Docker hub account at [Docker Hub](https://hub.docker.com/) if you don't already have one.

## Local development

:warning: Before you start developing, please ensure to comply with the following requirements:

1. You have understood the [principles of Kubernetes](https://kubernetes.io/docs/concepts/), and its [components](https://kubernetes.io/docs/concepts/overview/components/), what their purpose is and how they interact with each other.
1. You have understood the [architecture of the Machine Controller Manager](../design/README.md)

The development of the Machine Controller Manager could happen by targetting any cluster. You basically need a Kubernetes cluster running on a set of machines. You just need the [Kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/authenticate-across-clusters-kubeconfig/) file with the required access permissions attached to it.

### Installing the Machine Controller Manager locally
Clone the repository from GitHub.

```bash
$ git clone git@github.com:gardener/machine-controller-manager.git
$ cd machine-controller-manager
```

## Prepare the cluster

- Connect to the remote kubernetes cluster where you plan to deploy the Machine Controller Manager using kubectl. Set the environment variable KUBECONFIG to the path of the yaml file containing your cluster info
- Now, create the required CRDs on the remote cluster using the following command,
```bash
$ kubectl apply -f kubernetes/crds.yaml
```

## Getting started

- Create a `dev` directory.
- Copy the kubeconfig of kubernetes cluster where you wish to deploy the machines into `dev/target-kubeconfig.yaml`.
- (optional) Copy the kubeconfig of kubernetes cluster from where you wish to manage the machines into `dev/control-kubeconfig.yaml`. If you do this, also update the `Makefile` variable CONTROL_KUBECONFIG to point to `dev/control-kubeconfig.yaml` and CONTROL_NAMESPACE to the namespace in which your controller watches over.
- There is a rule dev in the `Makefile` which will automatically start the Machine Controller Manager with development settings:

```bash
$ make start
I1227 11:08:19.963638   55523 controllermanager.go:204] Starting shared informers
I1227 11:08:20.766085   55523 controller.go:247] Starting machine-controller-manager
```

The Machine Controller Manager should now be ready to manage the VMs in your kubernetes cluster.

:warning: The file `dev/target-kubeconfig.yaml` points to the cluster whose nodes you want to manage. `dev/control-kubeconfig.yaml` points to the cluster from where you want to manage the nodes from. However, `dev/control-kubeconfig.yaml` is optional.

## Testing Machine Classes

To test the creation/deletion of a single instance for one particular machine class you can use the `managevm` cli. The corresponding `INFRASTRUCTURE-machine-class.yaml` and the `INFRASTRUCTURE-secret.yaml` need to be defined upfront. To build and run it

```bash
go build -mod=vendor -o managevm cmd/machine-controller-manager-cli/main.go
# create machine
./managevm --secret PATH_TO/INFRASTRUCTURE-secret.yaml --machineclass PATH_TO/INFRASTRUCTURE-machine-class.yaml --classkind INFRASTRUCTURE --machinename test
# delete machine
./managevm --secret PATH_TO/INFRASTRUCTURE-secret.yaml --machineclass PATH_TO/INFRASTRUCTURE-machine-class.yaml --classkind INFRASTRUCTURE --machinename test --machineid INFRASTRUCTURE:///REGION/INSTANCE_ID
```

## Usage

To start using Machine Controller Manager, follow the links given at [usage here](../README.md).
