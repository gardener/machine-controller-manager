# Creating/Deleting machines (VM)
<!-- TOC -->

- [Creating/Deleting machines (VM)](#creatingdeleting-machines-vm)
  - [Setting up your usage environment](#setting-up-your-usage-environment)
  - [Important :](#important)
  - [Creating machine](#creating-machine)
  - [Inspect status of machine](#inspect-status-of-machine)
  - [Delete machine](#delete-machine)

<!-- /TOC -->
## Setting up your usage environment

* Follow the [steps described here](prerequisite.md)

## Important :

> Make sure that the `kubernetes/machine_objects/machine.yaml` points to the same class name as the `kubernetes/machine_classes/aws-machine-class.yaml`.

> Similarily `kubernetes/machine_objects/aws-machine-class.yaml` secret name and namespace should be same as that mentioned in `kubernetes/secrets/aws-secret.yaml`

## Creating machine

- Modify `kubernetes/machine_objects/machine.yaml` as per your requirement and create the VM as shown below:

```bash
$ kubectl apply -f kubernetes/machine_objects/machine.yaml
```

You should notice that the Machine Controller Manager has immediately picked up your manifest and started to create a new machine by talking to the cloud provider.

- Check Machine Controller Manager machines in the cluster

```bash
$ kubectl get machine
NAME           STATUS    AGE
test-machine   Running   5m
```

A new machine is created with the name provided in the `kubernetes/machine_objects/machine.yaml` file.

- After a few minutes (~3 minutes for AWS), you should notice a new node joining the cluster. You can verify this by running:

```bash
$ kubectl get nodes
NAME                                         STATUS     AGE     VERSION
ip-10-250-14-52.eu-east-1.compute.internal.  Ready      1m      v1.8.0
```

This shows that a new node has successfully joined the cluster.

## Inspect status of machine

To inspect the status of any created machine, run the command given below.

```bash
$ kubectl get machine test-machine -o yaml
```

```yaml
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"machine.sapcloud.io/v1alpha1","kind":"Machine","metadata":{"annotations":{},"labels":{"test-label":"test-label"},"name":"test-machine","namespace":""},"spec":{"class":{"kind":"AWSMachineClass","name":"test-aws"}}}
  clusterName: ""
  creationTimestamp: 2017-12-27T06:58:21Z
  finalizers:
  - machine.sapcloud.io/operator
  generation: 0
  initializers: null
  labels:
    node: ip-10-250-14-52.eu-east-1.compute.internal
    test-label: test-label
  name: test-machine
  namespace: ""
  resourceVersion: "12616948"
  selfLink: /apis/machine.sapcloud.io/v1alpha1/test-machine
  uid: 535e596c-ead3-11e7-a6c0-828f843e4186
spec:
  class:
    kind: AWSMachineClass
    name: test-aws
  providerID: aws:///eu-east-1/i-00bef3f2618ffef23
status:
  conditions:
  - lastHeartbeatTime: 2017-12-27T07:00:46Z
    lastTransitionTime: 2017-12-27T06:59:16Z
    message: kubelet has sufficient disk space available
    reason: KubeletHasSufficientDisk
    status: "False"
    type: OutOfDisk
  - lastHeartbeatTime: 2017-12-27T07:00:46Z
    lastTransitionTime: 2017-12-27T06:59:16Z
    message: kubelet has sufficient memory available
    reason: KubeletHasSufficientMemory
    status: "False"
    type: MemoryPressure
  - lastHeartbeatTime: 2017-12-27T07:00:46Z
    lastTransitionTime: 2017-12-27T06:59:16Z
    message: kubelet has no disk pressure
    reason: KubeletHasNoDiskPressure
    status: "False"
    type: DiskPressure
  - lastHeartbeatTime: 2017-12-27T07:00:46Z
    lastTransitionTime: 2017-12-27T07:00:06Z
    message: kubelet is posting ready status
    reason: KubeletReady
    status: "True"
    type: Ready
  currentStatus:
    lastUpdateTime: 2017-12-27T07:00:06Z
    phase: Running
  lastOperation:
    description: Machine is now ready
    lastUpdateTime: 2017-12-27T07:00:06Z
    state: Successful
    type: Create
  node: ip-10-250-14-52.eu-west-1.compute.internal
```

## Delete machine

To delete the VM using the `kubernetes/machine_objects/machine.yaml` as shown below

```bash
$ kubectl delete -f kubernetes/machine_objects/machine.yaml
```

Now the Machine Controller Manager picks up the manifest immediately and starts to delete the existing VM by talking to the cloud provider. The node should be detached from the cluster in a few minutes (~1min for AWS).
