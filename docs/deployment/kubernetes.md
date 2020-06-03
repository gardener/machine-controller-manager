# Deploying the Machine Controller Manager into a Kubernetes cluster

<!-- TOC -->

- [Deploying the Machine Controller Manager into a Kubernetes cluster](#deploying-the-machine-controller-manager-into-a-kubernetes-cluster)
	- [Prepare the cluster](#prepare-the-cluster)
	- [Build the Docker image](#build-the-docker-image)
	- [Configuring optional parameters while deploying](#configuring-optional-parameters-while-deploying)
	- [Usage](#usage)

<!-- /TOC -->

As already mentioned, the Machine Controller Manager is designed to run as controller in a Kubernetes cluster. The existing source code can be compiled and tested on a local machine as described in [Setting up a local development environment](../development/local_setup.md). You can deploy the Machine Controller Manager using the steps described below.

## Prepare the cluster

- Connect to the remote kubernetes cluster where you plan to deploy the Machine Controller Manager using the kubectl. Set the environment variable KUBECONFIG to the path of the yaml file containing the cluster info.
- Now, create the required CRDs on the remote cluster using the following command,
```bash
$ kubectl apply -f kubernetes/crds.yaml
```

## Build the Docker image

> :warning: Modify the `Makefile` to refer to your own registry.

- Run the build which generates the binary to `bin/machine-controller-manager`
```bash
$ make build
```
- Build docker image from latest compiled binary
```bash
$ make docker-image
```
- Push the last created docker image onto the online docker registry. 
```bash
$ make push
```

- Now you can deploy this docker image to your cluster. A sample development [file is given at](/kubernetes/deployment/in-tree/deployment.yaml). By default, the deployment manages the cluster it is running in. Optionally, the kubeconfig could also be passed as a flag as described in  `/kubernetes/deployment/in-tree/deployment.yaml`. This is done when you want your controller running outside the cluster to be managed from.
```bash
$ kubectl apply -f kubernetes/deployment/in-tree/deployment.yaml
```
- Also deploy the required clusterRole and clusterRoleBindings
```bash
$ kubectl apply -f kubernetes/deployment/in-tree/clusterrole.yaml
$ kubectl apply -f kubernetes/deployment/in-tree/clusterrolebinding.yaml
```

## Configuring optional parameters while deploying

Machine-controller-manager supports several configurable parameters while deploying. Refer to [the following lines](/kubernetes/deployment/in-tree/deployment.yaml#L21-L30), to know how each parameter can be configured, and what it's purpose is for.

## Usage

To start using Machine Controller Manager, follow the links given at [usage here](../README.md).