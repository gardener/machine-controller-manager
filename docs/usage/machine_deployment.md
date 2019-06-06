# Maintaining machine replicas using machines-deployments

<!-- TOC -->

- [Maintaining machine replicas using machines-deployments](#maintaining-machine-replicas-using-machines-deployments)
  - [Setting up your usage environment](#setting-up-your-usage-environment)
    - [Important :warning:](#important-warning)
  - [Creating machine-deployment](#creating-machine-deployment)
  - [Inspect status of machine-deployment](#inspect-status-of-machine-deployment)
  - [Health monitoring](#health-monitoring)
  - [Update your machines](#update-your-machines)
      - [Inspect existing cluster configuration](#inspect-existing-cluster-configuration)
      - [Perform a rolling update](#perform-a-rolling-update)
      - [Re-check cluster configuration](#re-check-cluster-configuration)
      - [More variants of updates](#more-variants-of-updates)
  - [Undo an update](#undo-an-update)
  - [Pause an update](#pause-an-update)
  - [Delete machine-deployment](#delete-machine-deployment)

<!-- /TOC -->

## Setting up your usage environment

Follow the [steps described here](prerequisite.md)

### Important :warning:

> Make sure that the `kubernetes/machine_objects/machine-deployment.yaml` points to the same class name as the `kubernetes/machine_classes/aws-machine-class.yaml`.

> Similarily `kubernetes/machine_classes/aws-machine-class.yaml` secret name and namespace should be same as that mentioned in `kubernetes/secrets/aws-secret.yaml`

## Creating machine-deployment

- Modify `kubernetes/machine_objects/machine-deployment.yaml` as per your requirement. Modify the number of replicas to the desired number of machines. Then, create an machine-deployment.

```bash
$ kubectl apply -f kubernetes/machine_objects/machine-deployment.yaml
```

Now the Machine Controller Manager picks up the manifest immediately and starts to create a new machines based on the number of replicas you have provided in the manifest.

- Check Machine Controller Manager machine-deployments in the cluster

```bash
$ kubectl get machinedeployment
NAME                      READY   DESIRED   UP-TO-DATE   AVAILABLE   AGE
test-machine-deployment   3       3         3            0           10m
```

You will notice a new machine-deployment with your given name

- Check Machine Controller Manager machine-sets in the cluster

```bash
$ kubectl get machineset
NAME                                 DESIRED   CURRENT   READY   AGE
test-machine-deployment-5bc6dd7c8f   3         3         0       10m
```

You will notice a new machine-set backing your machine-deployment

- Check Machine Controller Manager machines in the cluster

```bash
$ kubectl get machine
NAME                                       STATUS    AGE
test-machine-deployment-5bc6dd7c8f-5d24b   Pending   5m
test-machine-deployment-5bc6dd7c8f-6mpn4   Pending   5m
test-machine-deployment-5bc6dd7c8f-dpt2q   Pending   5m
```

Now you will notice N (number of replicas specified in the manifest) new machines whose name are prefixed with the machine-deployment object name that you created.

- After a few minutes (~3 minutes for AWS), you would see that new nodes have joined the cluster. You can see this using

```bash
$  kubectl get nodes
NAME                                          STATUS    AGE       VERSION
ip-10-250-20-19.eu-west-1.compute.internal    Ready     1m        v1.8.0
ip-10-250-27-123.eu-west-1.compute.internal   Ready     1m        v1.8.0
ip-10-250-31-80.eu-west-1.compute.internal    Ready     1m        v1.8.0
```

This shows how new nodes have joined your cluster

## Inspect status of machine-deployment

To inspect the status of any created machine-deployment run the command below,

```bash
$ kubectl get machinedeployment test-machine-deployment -o yaml
```

You should get the following output.

```yaml
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  annotations:
    deployment.kubernetes.io/revision: "1"
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"machine.sapcloud.io/v1alpha1","kind":"MachineDeployment","metadata":{"annotations":{},"name":"test-machine-deployment","namespace":""},"spec":{"minReadySeconds":200,"replicas":3,"selector":{"matchLabels":{"test-label":"test-label"}},"strategy":{"rollingUpdate":{"maxSurge":1,"maxUnavailable":1},"type":"RollingUpdate"},"template":{"metadata":{"labels":{"test-label":"test-label"}},"spec":{"class":{"kind":"AWSMachineClass","name":"test-aws"}}}}}
  clusterName: ""
  creationTimestamp: 2017-12-27T08:55:56Z
  generation: 0
  initializers: null
  name: test-machine-deployment
  namespace: ""
  resourceVersion: "12634168"
  selfLink: /apis/machine.sapcloud.io/v1alpha1/test-machine-deployment
  uid: c0b488f7-eae3-11e7-a6c0-828f843e4186
spec:
  minReadySeconds: 200
  replicas: 3
  selector:
    matchLabels:
      test-label: test-label
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
    type: RollingUpdate
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
  conditions:
  - lastTransitionTime: 2017-12-27T08:57:22Z
    lastUpdateTime: 2017-12-27T08:57:22Z
    message: Deployment has minimum availability.
    reason: MinimumReplicasAvailable
    status: "True"
    type: Available
  readyReplicas: 3
  replicas: 3
  updatedReplicas: 3
```


## Health monitoring

Health monitor is also applied similar to how it's described for [machine-sets](machine_set.md)

## Update your machines

Let us consider the scenario where you wish to update all nodes of your cluster from t2.xlarge machines to m5.xlarge machines. Assume that your current *test-aws* has its **spec.machineType: t2.xlarge** and your deployment *test-machine-deployment* points to this AWSMachineClass. 

#### Inspect existing cluster configuration

- Check Nodes present in the cluster

```bash
$ kubectl get nodes
NAME                                          STATUS    AGE       VERSION
ip-10-250-20-19.eu-west-1.compute.internal    Ready     1m        v1.8.0
ip-10-250-27-123.eu-west-1.compute.internal   Ready     1m        v1.8.0
ip-10-250-31-80.eu-west-1.compute.internal    Ready     1m        v1.8.0
```

- Check Machine Controller Manager machine-sets in the cluster. You will notice one machine-set backing your machine-deployment

```bash
$ kubectl get machineset
NAME                                 DESIRED   CURRENT   READY   AGE
test-machine-deployment-5bc6dd7c8f   3         3         3       10m
```

- Login to your cloud provider (AWS). In the VM management console, you will find N VMs created of type t2.xlarge.

#### Perform a rolling update

To update this machine-deployment VMs to `m5.xlarge`, we would do the following:

- Copy your existing aws-machine-class.yaml

```bash
cp kubernetes/machine_classes/aws-machine-class.yaml kubernetes/machine_classes/aws-machine-class-new.yaml
```

- Modify aws-machine-class-new.yaml, and update its *metadata.name: test-aws2* and  *spec.machineType: m5.xlarge*
- Now create this modified MachineClass

```bash
kubectl apply -f kubernetes/machine_classes/aws-machine-class-new.yaml
```

- Edit your existing machine-deployment

```bash
kubectl edit machinedeployment test-machine-deployment
```

- Update from *spec.template.spec.class.name: test-aws* to *spec.template.spec.class.name: test-aws2*

#### Re-check cluster configuration

After a few minutes (~3mins)

- Check nodes present in cluster now. They are different nodes.

```bash
$ kubectl get nodes
NAME                                          STATUS    AGE       VERSION
ip-10-250-11-171.eu-west-1.compute.internal   Ready     4m        v1.8.0
ip-10-250-17-213.eu-west-1.compute.internal   Ready     5m        v1.8.0
ip-10-250-31-81.eu-west-1.compute.internal    Ready     5m        v1.8.0
```

- Check Machine Controller Manager machine-sets in the cluster. You will notice two machine-sets backing your machine-deployment

```bash
$ kubectl get machineset
NAME                                 DESIRED   CURRENT   READY   AGE
test-machine-deployment-5bc6dd7c8f   0         0         0       1h
test-machine-deployment-86ff45cc5    3         3         3       20m
```

- Login to your cloud provider (AWS). In the VM management console, you will find N VMs created of type t2.xlarge in terminated state, and N new VMs of type m5.xlarge in running state.

This shows how a rolling update of a cluster from nodes with t2.xlarge to m5.xlarge went through.

#### More variants of updates

- The above demonstration was a simple use case. This could be more complex like - updating the system disk image versions/ kubelet versions/ security patches etc.
- You can also play around with the maxSurge and maxUnavailable fields in machine-deployment.yaml
- You can also change the update strategy from rollingupdate to recreate

## Undo an update

- Edit the existing machine-deployment

```bash
$ kubectl edit machinedeployment test-machine-deployment
```

- Edit the deployment to have this new field of *spec.rollbackTo.revision: 0* as shown as comments in `kubernetes/machine_objects/machine-deployment.yaml`
- This will undo your update to the previous version.

## Pause an update

- You can also pause the update while update is going on by editing the existing machine-deployment

```bash
$ kubectl edit machinedeployment test-machine-deployment
```

- Edit the deployment to have this new field of *spec.paused: true* as shown as comments in `kubernetes/machine_objects/machine-deployment.yaml`
- This will pause the rollingUpdate if it's in process

- To resume the update, edit the deployment as mentioned above and remove the field *spec.paused: true* updated earlier

## Delete machine-deployment

- To delete the VM using the `kubernetes/machine_objects/machine-deployment.yaml`

```bash
$ kubectl delete -f kubernetes/machine_objects/machine-deployment.yaml
```

The Machine Controller Manager picks up the manifest and starts to delete the existing VMs by talking to the cloud provider. The nodes should be detached from the cluster in a few minutes (~1min for AWS).
