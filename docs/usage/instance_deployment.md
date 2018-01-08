# Maintaining instance replicas using instances-deployments

## Setting up your usage environment

* Follow the [steps described here](prerequisite.md)

## Important :warning: 
- Make sure that the `kubernetes/instance-deployment.yaml` points to the same class name as the `kubernetes/aws-instance-class.yaml`.
- Similarily `kubernetes/aws-instance-class.yaml` secret name and namespace should be same as that mentioned in `kubernetes/secret.yaml`

## Creating instance-deployment

- Modify `kubernetes/instance-deployment.yaml` as per your requirement. Modify the number of replicas to the desired number of instances. Then, create an instance-deployment
```bash
$ kubectl apply -f kubernetes/instance-deployment.yaml
```
Now the Node Controller Manager picks up the manifest immediately and starts to create a new instances based on the number of replicas you have provided in the manifest.

- Check Node Controller Manager instance-deployments in the cluster
```bash
$ kubectl get instancedeployment
NAME                       KIND
test-instance-deployment   InstanceDeployment.v1alpha1.node.sapcloud.io
```
You will notice a new instance-deployment with your given name

- Check Node Controller Manager instance-sets in the cluster
```bash
$ kubectl get instanceset 
NAME                                  KIND
test-instance-deployment-5bc6dd7c8f   InstanceSet.v1alpha1.node.sapcloud.io
```
You will notice a new instance-set backing your instance-deployment

- Check Node Controller Manager instances in the cluster
```bash
$ kubectl get instance
NAME                                        KIND
test-instance-deployment-5bc6dd7c8f-5d24b   Instance.v1alpha1.node.sapcloud.io
test-instance-deployment-5bc6dd7c8f-6mpn4   Instance.v1alpha1.node.sapcloud.io
test-instance-deployment-5bc6dd7c8f-dpt2q   Instance.v1alpha1.node.sapcloud.io
```
Now you will notice N (number of replicas specified in the manifest) new instances whose name are prefixed with the instance-deployment object name that you created.

- After a few minutes (~3 minutes for AWS), you would see that new nodes have joined the cluster. You can see this using
```bash
$  kubectl get nodes
NAME                                          STATUS    AGE       VERSION
ip-10-250-20-19.eu-west-1.compute.internal    Ready     1m        v1.8.0
ip-10-250-27-123.eu-west-1.compute.internal   Ready     1m        v1.8.0
ip-10-250-31-80.eu-west-1.compute.internal    Ready     1m        v1.8.0
``` 
This shows how new nodes have joined your cluster

## Inspect status of instance-deployment

To inspect the status of any created instance-deployment run the command below, 
```bash
$ kubectl get instancedeployment test-instance-deployment -o yaml
apiVersion: node.sapcloud.io/v1alpha1
kind: InstanceDeployment
metadata:
  annotations:
    deployment.kubernetes.io/revision: "1"
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"node.sapcloud.io/v1alpha1","kind":"InstanceDeployment","metadata":{"annotations":{},"name":"test-instance-deployment","namespace":""},"spec":{"minReadySeconds":200,"replicas":3,"selector":{"matchLabels":{"test-label":"test-label"}},"strategy":{"rollingUpdate":{"maxSurge":1,"maxUnavailable":1},"type":"RollingUpdate"},"template":{"metadata":{"labels":{"test-label":"test-label"}},"spec":{"class":{"kind":"AWSInstanceClass","name":"test-aws"}}}}}
  clusterName: ""
  creationTimestamp: 2017-12-27T08:55:56Z
  generation: 0
  initializers: null
  name: test-instance-deployment
  namespace: ""
  resourceVersion: "12634168"
  selfLink: /apis/node.sapcloud.io/v1alpha1/test-instance-deployment
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
        kind: AWSInstanceClass
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

Health monitor is also applied similar to how it's described for [instance-sets](instance_set.md)

## Update your instances 

Let us consider the scenario where you wish to update all nodes of your cluster from t2.xlarge instances to m4.xlarge instances. Assume that your current *test-aws* has its **spec.instanceType: t2.xlarge** and your deployment *test-instance-deployment* points to this AWSInstanceClass. 

#### Inspect existing cluster configuration

- Check Nodes present in the cluster 
```bash
$  kubectl get nodes
NAME                                          STATUS    AGE       VERSION
ip-10-250-20-19.eu-west-1.compute.internal    Ready     1m        v1.8.0
ip-10-250-27-123.eu-west-1.compute.internal   Ready     1m        v1.8.0
ip-10-250-31-80.eu-west-1.compute.internal    Ready     1m        v1.8.0
``` 
- Check Node Controller Manager instance-sets in the cluster. You will notice one instance-set backing your instance-deployment
```bash
$ NAME                                  KIND
test-instance-deployment-5bc6dd7c8f   InstanceSet.v1alpha1.node.sapcloud.io
```
- Login to your cloud provider (AWS). In the VM management console, you will find N VMs created of type t2.xlarge.

#### Perform a rolling update

To update this instance-deployment VMs to m4.xlarge, we would do the following:

- Copy your existing aws-instance-class.yaml 
```bash
$ cp kubernetes/aws-instance-class.yaml kubernetes/aws-instance-class-new.yaml
```
- Modify aws-instance-class-new.yaml, and update its *metadata.name: test-aws2* and  *spec.instanceType: m4.xlarge*
- Now create this modified InstanceClass
```bash
$ kubectl apply -f kubernetes/aws-instance-class-new.yaml
```
- Edit your existing instance-deployment
```bash
$ kubectl edit instancedeployment test-instance-deployment
```
- Update from *spec.template.spec.class.name: test-aws* to *spec.template.spec.class.name: test-aws2*

#### Re-check cluster configuration

After a few minutes (~ 3mins)

- Check nodes present in cluster now. They are different nodes.
```bash
k get nodes
NAME                                          STATUS    AGE       VERSION
ip-10-250-11-171.eu-west-1.compute.internal   Ready     4m        v1.8.0
ip-10-250-17-213.eu-west-1.compute.internal   Ready     5m        v1.8.0
ip-10-250-31-81.eu-west-1.compute.internal    Ready     5m        v1.8.0
```
- Check Node Controller Manager instance-sets in the cluster. You will notice two instance-sets backing your instance-deployment
```bash
$ kubectl get instanceset
NAME                                  KIND
test-instance-deployment-5bc6dd7c8f   InstanceSet.v1alpha1.node.sapcloud.io
test-instance-deployment-86ff45cc5    InstanceSet.v1alpha1.node.sapcloud.io
```
- Login to your cloud provider (AWS). In the VM management console, you will find N VMs created of type t2.xlarge in terminated state, and N new VMs of type m4.xlarge in running state.

This shows how a rolling update of a cluster from nodes with t2.xlarge to m4.xlarge went through.

#### More variants of updates

- The above demonstration was a simple use case. This could be more complex like - updating the system disk image versions/ kubelet versions/ security patches etc.
- You can also play around with the maxSurge and maxUnavailable fields in instance-deployment.yaml
- You can also change the update strategy from rollingupdate to recreate 

## Undo an update

- Edit the existing instance-deployment
```bash
$ kubectl edit instancedeployment test-instance-deployment
```
- Edit the deployment to have this new field of *spec.rollbackTo.revision: 0* as shown as comments in `kubernetes/instance-deployment.yaml` 
- This will undo your update to the previous version.

## Pause an update

- You can also pause the update while update is going on by editing the existing instance-deployment 
```bash
$ kubectl edit instancedeployment test-instance-deployment
```
- Edit the deployment to have this new field of *spec.paused: true* as shown as comments in `kubernetes/instance-deployment.yaml` 
- This will pause the rollingUpdate if it's in process

- To resume the update, edit the deployment as mentioned above and remove the field *spec.paused: true* updated earlier

## Delete instance-deployment

- To delete the VM using the `kubernetes/instance-deployment.yaml`
```bash
$ kubectl delete -f kubernetes/instance-deployment.yaml
```

The Node Controller Manager picks up the manifest and starts to delete the existing VMs by talking to the cloud provider. The nodes should be detached from the cluster in a few minutes (~1min for AWS).