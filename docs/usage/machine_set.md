# Maintaining machine replicas using machines-sets

## Setting up your usage environment

* Follow the [steps described here](prerequisite.md)

## Important :warning: 
- Make sure that the `kubernetes/machine-set.yaml` points to the same class name as the `kubernetes/aws-machine-class.yaml`.
- Similarily `kubernetes/aws-machine-class.yaml` secret name and namespace should be same as that mentioned in `kubernetes/aws-secret.yaml`

## Creating machine-set

- Modify `kubernetes/machine-set.yaml` as per your requirement. You can modify the number of replicas to the desired number of machines. Then, create an machine-set
```bash
$ kubectl apply -f kubernetes/machine-set.yaml
```
You should notice that the Machine Controller Manager has immediately picked up your manifest and started to create a new machines based on the number of replicas you have provided in the manifest.

- Check Machine Controller Manager machine-sets in the cluster
```bash
$ kubectl get machineset
NAME                KIND
test-machine-set   MachineSet.v1alpha1.machine.sapcloud.io
```
You will see a new machine-set with your given name

- Check Machine Controller Manager machines in the cluster
```bash
$ kubectl get machine
NAME                      KIND
test-machine-set-b57zs   Machine.v1alpha1.machine.sapcloud.io
test-machine-set-c4bg8   Machine.v1alpha1.machine.sapcloud.io
test-machine-set-kvskg   Machine.v1alpha1.machine.sapcloud.io
```
Now you will see N (number of replicas specified in the manifest) new machines whose names are prefixed with the machine-set object name that you created.

- After a few minutes (~3 minutes for AWS), you should notice new nodes joining the cluster. You can verify this by running,
```bash
$ kubectl get nodes
NAME                                         STATUS    AGE       VERSION
ip-10-250-0-234.eu-west-1.compute.internal   Ready     3m        v1.8.0
ip-10-250-15-98.eu-west-1.compute.internal   Ready     3m        v1.8.0
ip-10-250-6-21.eu-west-1.compute.internal    Ready     2m        v1.8.0
``` 
This shows how new nodes have joined your cluster

## Inspect status of machine-set

- To inspect the status of any created machine-set run the following command,
```bash
$ kubectl get machineset test-machine-set -o yaml
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineSet
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"machine.sapcloud.io/v1alpha1","kind":"MachineSet","metadata":{"annotations":{},"name":"test-machine-set","namespace":"","test-label":"test-label"},"spec":{"minReadySeconds":200,"replicas":3,"selector":{"matchLabels":{"test-label":"test-label"}},"template":{"metadata":{"labels":{"test-label":"test-label"}},"spec":{"class":{"kind":"AWSMachineClass","name":"test-aws"}}}}}
  clusterName: ""
  creationTimestamp: 2017-12-27T08:37:42Z
  finalizers:
  - machine.sapcloud.io/operator
  generation: 0
  initializers: null
  name: test-machine-set
  namespace: ""
  resourceVersion: "12630893"
  selfLink: /apis/machine.sapcloud.io/v1alpha1/test-machine-set
  uid: 3469faaa-eae1-11e7-a6c0-828f843e4186
spec:
  machineClass: {}
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
        kind: AWSMachineClass
        name: test-aws
status:
  availableReplicas: 3
  fullyLabeledReplicas: 3
  machineSetCondition: null
  lastOperation:
    lastUpdateTime: null
  observedGeneration: 0
  readyReplicas: 3
  replicas: 3
```

## Health monitoring

- If you try to delete/terminate any of the machines backing the machine-set by either talking to the Machine Controller Manager or from the cloud provider, the Machine Controller Manager recreates a matching healthy machine to replace the deleted machine. 
- Similarly, if any of your machines are unreachable or in an unhealthy state (kubelet not ready / disk pressure) for longer than the configured timeout (~ 5mins), the Machine Controller Manager recreates the nodes to replace the unhealthy nodes.

## Delete machine-set

- To delete the VM using the `kubernetes/machine-set.yaml`
```bash
$ kubectl delete -f kubernetes/machine-set.yaml
```
Now the Machine Controller Manager has immediately picked up your manifest and started to delete the existing VMs by talking to the cloud provider. Your nodes should be detached from the cluster in a few minutes (~1min for AWS).