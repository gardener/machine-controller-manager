# Creating/Deleting instances (VM)

## Setting up your usage environment

* Follow the [steps described here](prerequisite.md)

## Important :warning: 
- Make sure that the `kubernetes/instance.yaml` points to the same class name as the `kubernetes/aws-instance-class.yaml`.
- Similarily `kubernetes/aws-instance-class.yaml` secret name and namespace should be same as that mentioned in `kubernetes/secret.yaml`

## Creating instance

- Modify `kubernetes/instance.yaml` as per your requirement and create the VM as shown below
```bash
$ kubectl apply -f kubernetes/instance.yaml
```
You should notice that the Node Controller Manager has immediately picked up your manifest and started to create a new instance by talking to the cloud provider.

- Check Node Controller Manager instances in the cluster
```bash
$ kubectl get instance
test-instance	Instance.v1alpha1.node.sapcloud.io
```
A new instance is created with the name provided in the `kubernetes/instance.yaml` file.

- After a few minutes (~3 minutes for AWS), you should notice a new node joining the cluster. You can verify this by running,
```bash
$ kubectl get nodes
NAME                                         STATUS     AGE     VERSION
ip-10-250-14-52.eu-east-1.compute.internal.  Ready      1m      v1.8.0
``` 
This shows that a new node has successfully joined the cluster.

## Inspect status of instance

- To inspect the status of any created instance, run the command given below.
```bash
$ kubectl get instance test-instance -o yaml
apiVersion: node.sapcloud.io/v1alpha1
kind: Instance
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"node.sapcloud.io/v1alpha1","kind":"Instance","metadata":{"annotations":{},"labels":{"test-label":"test-label"},"name":"test-instance","namespace":""},"spec":{"class":{"kind":"AWSInstanceClass","name":"test-aws"}}}
  clusterName: ""
  creationTimestamp: 2017-12-27T06:58:21Z
  finalizers:
  - node.sapcloud.io/operator
  generation: 0
  initializers: null
  labels:
    node: ip-10-250-14-52.eu-east-1.compute.internal
    test-label: test-label
  name: test-instance
  namespace: ""
  resourceVersion: "12616948"
  selfLink: /apis/node.sapcloud.io/v1alpha1/test-instance
  uid: 535e596c-ead3-11e7-a6c0-828f843e4186
spec:
  class:
    kind: AWSInstanceClass
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
    description: Instance is now ready
    lastUpdateTime: 2017-12-27T07:00:06Z
    state: Successful
    type: Create
  node: ip-10-250-14-52.eu-west-1.compute.internal
```

## Delete instance

- To delete the VM using the `kubernetes/instance.yaml` as shown below
```bash
$ kubectl delete -f kubernetes/instance.yaml
```
Now the Node Controller Manager picks up the manifest immediately and starts to delete the existing VM by talking to the cloud provider. The node should be detached from the cluster in a few minutes (~1min for AWS).