# Setting up the usage environment
<!-- TOC -->

- [Setting up the usage environment](#setting-up-the-usage-environment)
	- [Important :warning:](#important-warning)
	- [Set KUBECONFIG](#set-kubeconfig)
	- [Replace provider credentials and desired VM configurations](#replace-provider-credentials-and-desired-vm-configurations)
	- [Deploy required CRDs and Objects](#deploy-required-crds-and-objects)
	- [Check current cluster state](#check-current-cluster-state)

<!-- /TOC -->
## Important :warning:

> All paths are relative to the root location of this project repository.

> Run the Machine Controller Manager either as described in [Setting up a local development environment](../development/local_setup.md) or [Deploying the Machine Controller Manager into a Kubernetes cluster](../deployment/kubernetes.md).

> Make sure that the following steps are run before managing machines/ machine-sets/ machine-deploys.

## Set KUBECONFIG

Using the existing [Kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/authenticate-across-clusters-kubeconfig/), open another Terminal panel/window with the `KUBECONFIG` environment variable pointing to this Kubeconfig file as shown below,

```bash
$ export KUBECONFIG=<PATH_TO_REPO>/dev/kubeconfig.yaml
```

## Replace provider credentials and desired VM configurations

Open `kubernetes/machine_classes/aws-machine-class.yaml` and replace required values there with the desired VM configurations. 

Similarily open `kubernetes/secrets/aws-secret.yaml` and replace - *userData, providerAccessKeyId, providerSecretAccessKey* with base64 encoded values of cloudconfig file, AWS access key id, and AWS secret access key respectively. Use the following command to get the base64 encoded value of your details

```bash
$ echo "sample-cloud-config" | base64
base64-encoded-cloud-config
```

Do the same for your access key id and secret access key.

## Deploy required CRDs and Objects

Create all the required CRDs in the cluster using `kubernetes/crds.yaml`
```bash
$ kubectl apply -f kubernetes/crds.yaml
```

Create the class template that will be used as an machine template to create VMs using `kubernetes/machine_classes/aws-machine-class.yaml`
```bash
$ kubectl apply -f kubernetes/machine_classes/aws-machine-class.yaml
```

Create the secret used for the cloud credentials and cloudconfig using `kubernetes/secrets/aws-secret.yaml`
```bash
$ kubectl apply -f kubernetes/secrets/aws-secret.yaml
```

## Check current cluster state

Get to know the current cluster state using the following commands,

- Checking aws-machine-class in the cluster

```bash
$ kubectl get awsmachineclass
NAME       MACHINE TYPE   AMI          AGE
test-aws   t2.large       ami-123456   5m
```

- Checking kubernetes secrets in the cluster

```bash
$ kubectl get secret
NAME                  TYPE                                  DATA      AGE
test-secret           Opaque                                3         21h
```

- Checking kubernetes nodes in the cluster

```bash
$ kubectl get nodes
```

Lists the default set of nodes attached to your cluster

- Checking Machine Controller Manager machines in the cluster

```bash
$ kubectl get machine
No resources found.
```

- Checking Machine Controller Manager machine-sets in the cluster

```bash
$ kubectl get machineset
No resources found.
```

- Checking Machine Controller Manager machine-deploys in the cluster

```bash
$ kubectl get machinedeployment
No resources found.
```