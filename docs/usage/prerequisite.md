# Setting up the usage environment

:warning: All paths are relative to the root location of this project repository.

- Run the Node Controller Manager either as described in [Setting up a local development environment](../development/local_setup.md) or [Deploying the Node Controller Manager into a Kubernetes cluster](../deployment/kubernetes.md).
- Make sure that the following steps are run before managing instances/ instance-sets/ instance-deploys.

## Set KUBECONFIG

Using the existing [Kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/authenticate-across-clusters-kubeconfig/), open another Terminal panel/window with the `KUBECONFIG` environment variable pointing to this Kubeconfig file as shown below,

```bash
$ export KUBECONFIG=$GOPATH/src/github.com/gardener/node-controller-manager/dev/kubeconfig.yaml
```

## Replace provider credentials and desired VM configurations

Open `kubernetes/aws-instance-class.yaml` and replace required values there with the desired VM configurations. 

Similarily open `kubernetes/secret.yaml` and replace - *userData, providerAccessKeyId, providerSecretAccessKey* with base64 encoded values of cloudconfig file, AWS access key id, and AWS secret access key respectively. Use the following command to get the base64 encoded value of your details

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

Create the class template that will be used as an instance template to create VMs using `kubernetes/aws-instance-class.yaml`
```bash
$ kubectl apply -f kubernetes/aws-instance-class.yaml
```

Create the secret used for the cloud credentials and cloudconfig using `kubernetes/secret.yaml`
```bash
$ kubectl apply -f kubernetes/secret.yaml
```

## Check current cluster state

Get to know the current cluster state using the following commands,

- Checking aws-instance-class in the cluster
```bash
$ kubectl get awsinstanceclass
NAME       KIND
test-aws   AWSInstanceClass.v1alpha1.node.sapcloud.io
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

- Checking Node Controller Manager instances in the cluster
```bash
$ kubectl get instance
No resources found.
```

- Checking Node Controller Manager instance-sets in the cluster
```bash
$ kubectl get instanceset
No resources found.
```

- Checking Node Controller Manager instance-deploys in the cluster
```bash
$ kubectl get instancedeployment
No resources found.
```