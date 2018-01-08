# Maintaining instance replicas using instances-sets

## Setting up your usage environment

* Follow the [steps described here](prerequisite.md)

## Important :warning: 
- Make sure that the `kubernetes/instance-set.yaml` points to the same class name as the `kubernetes/aws-instance-class.yaml`.
- Similarily `kubernetes/aws-instance-class.yaml` secret name and namespace should be same as that mentioned in `kubernetes/secret.yaml`

## Creating instance-set

- Modify `kubernetes/instance-set.yaml` as per your requirement. You can modify the number of replicas to the desired number of instances. Then, create an instance-set
```bash
$ kubectl apply -f kubernetes/instance-set.yaml
```
You should notice that the Node Controller Manager has immediately picked up your manifest and started to create a new instances based on the number of replicas you have provided in the manifest.

- Check Node Controller Manager instance-sets in the cluster
```bash
$ kubectl get instanceset
NAME                KIND
test-instance-set   InstanceSet.v1alpha1.node.sapcloud.io
```
You will see a new instance-set with your given name

- Check Node Controller Manager instances in the cluster
```bash
$ kubectl get instance
NAME                      KIND
test-instance-set-b57zs   Instance.v1alpha1.node.sapcloud.io
test-instance-set-c4bg8   Instance.v1alpha1.node.sapcloud.io
test-instance-set-kvskg   Instance.v1alpha1.node.sapcloud.io
```
Now you will see N (number of replicas specified in the manifest) new instances whose names are prefixed with the instance-set object name that you created.

- After a few minutes (~3 minutes for AWS), you should notice new nodes joining the cluster. You can verify this by running,
```bash
$ kubectl get nodes
NAME                                         STATUS    AGE       VERSION
ip-10-250-0-234.eu-west-1.compute.internal   Ready     3m        v1.8.0
ip-10-250-15-98.eu-west-1.compute.internal   Ready     3m        v1.8.0
ip-10-250-6-21.eu-west-1.compute.internal    Ready     2m        v1.8.0
``` 
This shows how new nodes have joined your cluster

## Inspect status of instance-set

- To inspect the status of any created instance-set run the following command,
```bash
$ kubectl get instanceset test-instance-set -o yaml
apiVersion: node.sapcloud.io/v1alpha1
kind: InstanceSet
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"node.sapcloud.io/v1alpha1","kind":"InstanceSet","metadata":{"annotations":{},"name":"test-instance-set","namespace":"","test-label":"test-label"},"spec":{"minReadySeconds":200,"replicas":3,"selector":{"matchLabels":{"test-label":"test-label"}},"template":{"metadata":{"labels":{"test-label":"test-label"}},"spec":{"class":{"kind":"AWSInstanceClass","name":"test-aws"}}}}}
  clusterName: ""
  creationTimestamp: 2017-12-27T08:37:42Z
  finalizers:
  - node.sapcloud.io/operator
  generation: 0
  initializers: null
  name: test-instance-set
  namespace: ""
  resourceVersion: "12630893"
  selfLink: /apis/node.sapcloud.io/v1alpha1/test-instance-set
  uid: 3469faaa-eae1-11e7-a6c0-828f843e4186
spec:
  instanceClass: {}
  minReadySeconds: 200
  replicas: 3
  selector:
    matchLabels:
      test-label: test-label
  template:
    metadata:
      creationTimestamp: null
      labels:
        test-label: test-label
    spec:
      class:
        kind: AWSInstanceClass
        name: test-aws
status:
  availableReplicas: 3
  fullyLabeledReplicas: 3
  instanceSetCondition: null
  lastOperation:
    lastUpdateTime: null
  observedGeneration: 0
  readyReplicas: 3
  replicas: 3
```

## Health monitoring

- If you try to delete/terminate any of the instances backing the instance-set by either talking to the Node Controller Manager or from the cloud provider, the Node Controller Manager recreates a matching healthy instance to replace the deleted instance. 
- Similarly, if any of your instances are unreachable or in an unhealthy state (kubelet not ready / disk pressure) for longer than the configured timeout (~ 5mins), the Node Controller Manager recreates the nodes to replace the unhealthy nodes.

## Delete instance-set

- To delete the VM using the `kubernetes/instance-set.yaml`
```bash
$ kubectl delete -f kubernetes/instance-set.yaml
```
Now the Node Controller Manager has immediately picked up your manifest and started to delete the existing VMs by talking to the cloud provider. Your nodes should be detached from the cluster in a few minutes (~1min for AWS).