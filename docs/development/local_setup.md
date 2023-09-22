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
1. You have understood the [architecture of the Machine Controller Manager](../../README.md#design-of-machine-controller-manager)

The development of the Machine Controller Manager could happen by targeting any cluster. You basically need a Kubernetes cluster running on a set of machines. You just need the [Kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/authenticate-across-clusters-kubeconfig/) file with the required access permissions attached to it.

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

**Setup and Restore with Gardener**

_Setup_

In gardener access to static kubeconfig files is no longer supported due to security reasons. One needs to generate short-lived (max TTL = 1 day) admin kube configs for target and control clusters.
A convenience script/Makefile target has been provided to do the required initial setup which includes:
* Creating a temporary directory where target and control kubeconfigs will be stored.
* Create a request to generate the short lived admin kubeconfigs. These are downloaded and stored in the temporary folder created above.
* In gardener clusters `DWD (Dependency Watchdog)` runs as an additional component which can interfere when MCM/CA is scaled down. To prevent that an annotation `dependency-watchdog.gardener.cloud/ignore-scaling` is added to `machine-controller-manager` deployment which prevents `DWD` from scaling up the deployment replicas.
* Scales down `machine-controller-manager` deployment in the control cluster to 0 replica.
* Creates the required `.env` file and populates required environment variables which are then used by the `Makefile` in both `machine-controller-manager` and in `machine-controller-manager-provider-<provider-name>` projects.
* Copies the generated and downloaded kubeconfig files for the target and control clusters to `machine-controller-manager-provider-<provider-name>` project as well.

To do the above you can either invoke `make gardener-setup` or you can directly invoke the script `./hack/gardener_local_setup.sh`. If you invoke the script with `-h or --help` option then it will give you all CLI options that one can pass. 

_Restore_

Once the testing is over you can invoke a convenience script/Makefile target which does the following:
* Removes all generated admin kubeconfig files from both `machine-controller-manager` and in `machine-controller-manager-provider-<provider-name>` projects.
* Removes the `.env` file that was generated as part of the setup from both `machine-controller-manager` and in `machine-controller-manager-provider-<provider-name>` projects.
* Scales up `machine-controller-manager` deployment in the control cluster back to 1 replica.
* Removes the annotation `dependency-watchdog.gardener.cloud/ignore-scaling` that was added to prevent `DWD` to scale up MCM.

To do the above you can either invoke `make gardener-restore` or you can directly invoke the script `./hack/gardener_local_restore.sh`. If you invoke the script with `-h or --help` option then it will give you all CLI options that one can pass.

**Setup and Restore without Gardener**

_Setup_

If you are not running MCM components in a gardener cluster, then it is assumed that there is not going to be any `DWD (Dependency Watchdog)` component.
A convenience script/Makefile target has been provided to the required initial setup which includes:
* Copies the provided control and target kubeconfig files to `machine-controller-manager-provider-<provider-name>` project.
* Scales down `machine-controller-manager` deployment in the control cluster to 0 replica.
* Creates the required `.env` file and populates required environment variables which are then used by the `Makefile` in both `machine-controller-manager` and in `machine-controller-manager-provider-<provider-name>` projects.

To do the above you can either invoke `make non-gardener-setup` or you can directly invoke the script `./hack/non_gardener_local_setup.sh`. If you invoke the script with `-h or --help` option then it will give you all CLI options that one can pass.

_Restore_

Once the testing is over you can invoke a convenience script/Makefile target which does the following:
* Removes all provided kubeconfig files from both `machine-controller-manager` and in `machine-controller-manager-provider-<provider-name>` projects.
* Removes the `.env` file that was generated as part of the setup from both `machine-controller-manager` and in `machine-controller-manager-provider-<provider-name>` projects.
* Scales up `machine-controller-manager` deployment in the control cluster back to 1 replica.

To do the above you can either invoke `make non-gardener-restore` or you can directly invoke the script `./hack/non_gardener_local_restore.sh`. If you invoke the script with `-h or --help` option then it will give you all CLI options that one can pass.

Once the setup is done then you can start the `machine-controller-manager` as a local process using the following `Makefile` target:

```bash
$ make start
I1227 11:08:19.963638   55523 controllermanager.go:204] Starting shared informers
I1227 11:08:20.766085   55523 controller.go:247] Starting machine-controller-manager
```

:warning: The file `dev/target-kubeconfig.yaml` points to the cluster whose nodes you want to manage. `dev/control-kubeconfig.yaml` points to the cluster from where you want to manage the nodes from. However, `dev/control-kubeconfig.yaml` is optional.

The Machine Controller Manager should now be ready to manage the VMs in your kubernetes cluster.

:warning: This is assuming that your MCM is built to manage machines for any in-tree supported providers. There is a new way to deploy and manage out of tree (external) support for providers whose development can be [found here](cp_support_new.md)

## Testing Machine Classes

To test the creation/deletion of a single instance for one particular machine class you can use the `managevm` cli. The corresponding `INFRASTRUCTURE-machine-class.yaml` and the `INFRASTRUCTURE-secret.yaml` need to be defined upfront. To build and run it

```bash
GO111MODULE=on go build -mod=vendor -o managevm cmd/machine-controller-manager-cli/main.go
# create machine
./managevm --secret PATH_TO/INFRASTRUCTURE-secret.yaml --machineclass PATH_TO/INFRASTRUCTURE-machine-class.yaml --classkind INFRASTRUCTURE --machinename test
# delete machine
./managevm --secret PATH_TO/INFRASTRUCTURE-secret.yaml --machineclass PATH_TO/INFRASTRUCTURE-machine-class.yaml --classkind INFRASTRUCTURE --machinename test --machineid INFRASTRUCTURE:///REGION/INSTANCE_ID
```

## Usage

To start using Machine Controller Manager, follow the links given at [usage here](../../README.md).
