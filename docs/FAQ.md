---
title: Machine Controller Manager FAQ
description: Commonly asked questions about MCM
---

# Frequently Asked Questions

The answers in this FAQ apply to the newest (HEAD) version of Machine Controller Manager. If
you're using an older version of MCM please refer to corresponding version of
this document. Few of the answers assume that the MCM being used is in conjuction with [cluster-autoscaler](https://github.com/gardener/autoscaler):

# Table of Contents:
<!--- TOC BEGIN -->
* [Basics](#basics)
  * [What is Machine Controller Manager?](#what-is-machine-controller-manager)
  * [Why is my machine deleted?](#Why-is-my-machine-deleted)
  * [What are the different sub-controllers in MCM?](#What-are-the-different-sub-controllers-in-MCM)
  * [What is Safety Controller in MCM?](#What-is-safety-controller-in-MCM)

* [How to?](#how-to)
  * [How to install MCM in a Kubernetes cluster?](#How-to-install-MCM-in-a-kubernetes-cluster)
  * [How to better control the rollout process of the worker nodes?](#How-to-better-control-the-rollout-process-of-the-worker-nodes)
  * [How to scale down MachineDeployment by selective deletion of machines?](#How-to-scale-down-machinedeployment-by-selective-deletion-of-machines)
  * [How to force delete a machine?](#How-to-force-delete-a-machine)
  * [How to pause the ongoing rolling-update of the machinedeployment?](#How-to-pause-the-ongoing-rolling-update-of-the-machinedeployment)
  * [How to delete machine object immedietly if I don't have access to it?](#how-to-delete-machine-object-immedietly-if-i-dont-have-access-to-it)
  * [How to avoid garbage collection of your node?](#How-to-avoid-garbage-collection-of-your-node)
  * [How to trigger rolling update of a machinedeployment?](#how-to-trigger-rolling-update-of-a-machinedeployment)

* [Internals](#internals)
  * [What is the high level design of MCM?](#What-is-the-high-level-design-of-MCM)
  * [What are the different configuration options in MCM?](#What-are-the-different-configuration-options-in-MCM)
  * [What are the different timeouts/configurations in a machine's lifecycle?](#What-are-the-different-timeoutsconfigurations-in-a-machines-lifecycle)
  * [How is the drain of a machine implemented?](#How-is-the-drain-of-a-machine-implemented)
  * [How are the stateful applications drained during machine deletion?](#How-are-the-stateful-applications-drained-during-machine-deletion)
  * [How does maxEvictRetries configuration work with drainTimeout configuration?](#How-does-maxEvictRetries-configuration-work-with-drainTimeout-configuration)
  * [What are the different phases of a machine?](#What-are-the-different-phases-of-a-machine)
  * [What health checks are performed on a machine?](#what-health-checks-are-performed-on-a-machine)
  * [How does rate limiting replacement of machine work in MCM ? How is it related to meltdown protection?](#how-does-rate-limiting-replacement-of-machine-work-in-mcm-how-is-it-related-to-meltdown-protection)
  * [How MCM responds when scale-out/scale-in is done during rolling update of a machinedeployment?](#how-mcm-responds-when-scale-outscale-in-is-done-during-rolling-update-of-a-machinedeployment)
  * [How some unhealthy machines are drained quickly?](#how-some-unhealthy-machines-are-drained-quickly-)
  * [How does MCM prioritize the machines for deletion on scale-down of machinedeployment?](#how-does-mcm-prioritize-the-machines-for-deletion-on-scale-down-of-machinedeployment)

* [Troubleshooting](#troubleshooting)
  * [My machine is stuck in deletion for 1 hr, why?](#My-machine-is-stuck-in-deletion-for-1-hr-why)
  * [My machine is not joining the cluster, why?](#My-machine-is-not-joining-the-cluster-why)
* [Developer](#developer)
  * [How should I test my code before submitting a PR?](#How-should-I-test-my-code-before-submitting-a-PR)
  * [I need to change the APIs, what are the recommended steps?](#I-need-to-change-the-APIs-what-are-the-recommended-steps)
  * [How can I update the dependencies of MCM?](#How-can-I-update-the-dependencies-of-MCM)
* [In the context of Gardener](#in-the-context-of-gardener)
  * [How can I configure MCM using Shoot resource?](#How-can-I-configure-MCM-using-Shoot-resource)
  * [How is my worker-pool spread across zones?](#How-is-my-worker-pool-spread-across-zones)

<!--- TOC END -->

# Basics

### What is Machine Controller Manager?

Machine Controller Manager aka MCM is a bunch of controllers used for the lifecycle management of the worker machines. It reconciles a set of CRDs such as `Machine`, `MachineSet`, `MachineDeployment` which depicts the functionality of `Pod`, `Replicaset`, `Deployment` of the core Kubernetes respectively. Read more about it at [README](https://github.com/gardener/machine-controller-manager/tree/master/docs).

* Gardener uses MCM to manage its Kubernetes nodes of the shoot cluster. However, by design, MCM can be used independent of Gardener.

### Why is my machine deleted?

A machine is deleted by MCM generally for 2 reasons-

- Machine is unhealthy for at least `MachineHealthTimeout` period. The default `MachineHealthTimeout` is 10 minutes.

   * By default, a machine is considered unhealthy if any of the following node conditions - `DiskPressure`, `KernelDeadlock`, `FileSystem`, `Readonly` is set to `true`, or `KubeletReady` is set to `false`. However, this is something that is configurable using the following [flag](../kubernetes/deployment/out-of-tree/deployment.yaml#L30).

- Machine is scaled down by the `MachineDeployment` resource.

   * This is very usual when an external controller cluster-autoscaler (aka CA) is used with MCM. CA deletes the under-utilized machines by scaling down the `MachineDeployment`. Read more about cluster-autoscaler's scale down behavior [here](https://github.com/gardener/autoscaler/blob/machine-controller-manager-provider/cluster-autoscaler/FAQ.md#how-does-scale-down-work).

### What are the different sub-controllers in MCM?

MCM mainly contains the following sub-controllers:

* `MachineDeployment Controller`: Responsible for reconciling the `MachineDeployment` objects. It manages the lifecycle of the `MachineSet` objects.
* `MachineSet Controller`: Responsible for reconciling the `MachineSet` objects. It manages the lifecycle of the `Machine` objects.
* `Machine Controller`: responsible for reconciling the `Machine` objects. It manages the lifecycle of the actual VMs/machines created in cloud/on-prem. This controller has been moved out of tree. Please refer an AWS machine controller for more info - [link](https://github.com/gardener/machine-controller-manager-provider-gcp).
* Safety-controller: Responsible for handling the unidentified/unknown behaviors from the cloud providers. Please read more about its functionality [below](#what-is-safety-controller).

### What is Safety Controller in MCM?

`Safety Controller` contains following functions:

* Orphan VM handler:
  * It lists all the VMs in the cloud matching the `tag` of given cluster name and maps the VMs with the `machine` objects using the `ProviderID` field. VMs without any backing `machine` objects are logged and deleted after confirmation.
  * This handler runs every 30 minutes and is configurable via [machine-safety-orphan-vms-period](https://github.com/gardener/machine-controller-manager/blob/master/cmd/machine-controller-manager/app/options/options.go#L112) flag.
* Freeze mechanism: 
  * `Safety Controller` freezes the `MachineDeployment` and `MachineSet` controller if the number of `machine` objects goes beyond a certain threshold on top of `Spec.Replicas`. It can be configured by the flag [--safety-up or --safety-down](https://github.com/gardener/machine-controller-manager/blob/master/cmd/machine-controller-manager/app/options/options.go#L102-L103) and also [machine-safety-overshooting-period](https://github.com/gardener/machine-controller-manager/blob/master/cmd/machine-controller-manager/app/options/options.go#L113).
  * `Safety Controller` freezes the functionality of the MCM if either of the `target-apiserver` or the `control-apiserver` is not reachable.
  * `Safety Controller` unfreezes the MCM automatically once situation is resolved to normal. A `freeze` label is applied on `MachineDeployment`/`MachineSet` to enforce the freeze condition.

# How to?

### How to install MCM in a Kubernetes cluster?

MCM can be installed in a cluster with following steps:

* Apply all the CRDs from [here](https://github.com/gardener/machine-controller-manager/tree/master/kubernetes/crds)
* Apply all the deployment, role-related objects from [here](https://github.com/gardener/machine-controller-manager/tree/master/kubernetes/deployment/out-of-tree).

  * Control cluster is the one where the `machine-*` objects are stored. Target cluster is where all the node objects are registered.

### How to better control the rollout process of the worker nodes?

MCM allows configuring the rollout of the worker machines using `maxSurge` and `maxUnavailable` fields. These fields are applicable only during the rollout process and means nothing in general scale up/down scenarios.
The overall process is very similar to how the `Deployment Controller` manages pods during `RollingUpdate`.

* `maxSurge` refers to the number of additional machines that can be added on top of the `Spec.Replicas` of MachineDeployment _during rollout process_.
* `maxUnavailable` refers to the number of machines that can be deleted from `Spec.Replicas` field of the MachineDeployment _during rollout process_.


### How to scale down MachineDeployment by selective deletion of machines?

During scale down, triggered via `MachineDeployment`/`MachineSet`, MCM prefers to delete the `machine/s` which have the least priority set.
Each `machine` object has an annotation `machinepriority.machine.sapcloud.io` set to `3` by default. Admin can reduce the priority of the given machines by changing the annotation value to `1`. The next scale down by `MachineDeployment` shall delete the machines with the least priority first.

### How to force delete a machine?

A machine can be force deleted by adding the label `force-deletion: "True"` on the `machine` object before executing the actual delete command. During force deletion, MCM skips the drain function and simply triggers the deletion of the machine. This label should be used with caution as it can violate the PDBs for pods running on the machine.

### How to pause the ongoing rolling-update of the machinedeployment?

An ongoing rolling-update of the machine-deployment can be paused by using `spec.paused` field. See the example below:
```
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: test-machine-deployment
spec:
  paused: true
```

It can be unpaused again by removing the `Paused` field from the machine-deployment.

### How to delete machine object immedietly if I don't have access to it?

If the user doesn't have access to the machine objects (like in case of Gardener clusters) and they would like to replace a node immedietly then they can place the annotation `node.machine.sapcloud.io/trigger-deletion-by-mcm: "true"` on their node. This will start the replacement of the machine with a new node.

On the other hand if the user deletes the node object immedietly then replacement will start only after `MachineHealthTimeout`.

This annotation can also be used if the user wants to expedite the [replacement of unhealthy nodes](#how-does-rate-limiting-replacement-of-machine-work-in-mcm-how-is-it-related-to-meltdown-protection)

`NOTE`: 
- `node.machine.sapcloud.io/trigger-deletion-by-mcm: "false"` annotation is NOT acted upon by MCM , neither does it mean that MCM will not replace this machine.
- this annotation would delete the desired machine but another machine would be created to maintain `desired replicas` specified for the machineDeployment/machineSet. Currently if the user doesn't have access to machineDeployment/machineSet then they cannot remove a machine without replacement.

### How to avoid garbage collection of your node?

MCM provides an in-built safety mechanism to garbage collect VMs which have no corresponding machine object. This is done to save costs and is one of the key features of MCM.
However, sometimes users might like to add nodes directly to the cluster without the help of MCM and would prefer MCM to not garbage collect such VMs. 
To do so they should remove/not-use tags on their VMs containing the following strings:

1) `kubernetes.io/cluster/`
2) `kubernetes.io/role/`
3) `kubernetes-io-cluster-`
4) `kubernetes-io-role-`

### How to trigger rolling update of a machinedeployment?

Rolling update can be triggered for a machineDeployment by updating one of the following:
  * `.spec.template.annotations`
  * `.spec.template.spec.class.name`

# Internals
### What is the high level design of MCM?

Please refer the following [document](../README.md#design-of-machine-controller-manager).

### What are the different configuration options in MCM?

MCM allows configuring many knobs to fine-tune its behavior according to the user's need. 
Please refer to the [link](https://github.com/gardener/machine-controller-manager/blob/master/cmd/machine-controller-manager/app/options/options.go) to check the exact configuration options.

### What are the different timeouts/configurations in a machine's lifecycle?

A machine's lifecycle is governed by mainly following timeouts, which can be configured [here](https://github.com/gardener/machine-controller-manager/blob/master/kubernetes/machine_objects/machine-deployment.yaml#L30-L34).

* `MachineDrainTimeout`: Amount of time after which drain times out and the machine is force deleted. Default ~2 hours.
* `MachineHealthTimeout`: Amount of time after which an unhealthy machine is declared `Failed` and the machine is replaced by `MachineSet` controller.
* `MachineCreationTimeout`: Amount of time after which a machine creation is declared `Failed` and the machine is replaced by the `MachineSet` controller.
* `NodeConditions`: List of node conditions which if set to true for `MachineHealthTimeout` period, the machine is declared `Failed` and replaced by `MachineSet` controller.
* `MaxEvictRetries`: An integer number depicting the number of times a failed _eviction_ should be retried on a pod during drain process. A pod is _deleted_ after `max-retries`.

### How is the drain of a machine implemented?

MCM imports the functionality from the upstream Kubernetes-drain library. Although, few parts have been modified to make it work best in the context of MCM. Drain is executed before machine deletion for graceful migration of the applications. 
Drain internally uses the `EvictionAPI` to evict the pods and triggers the `Deletion` of pods after `MachineDrainTimeout`. Please note:

* Stateless pods are evicted in parallel.
* Stateful applications (with PVCs) are serially evicted. Please find more info in this [answer below](#how-are-the-stateful-applications-drained-during-machine-deletion).


### How are the stateful applications drained during machine deletion?

Drain function serially evicts the stateful-pods. It is observed that serial eviction of stateful pods yields better overall availability of pods as the underlying cloud in most cases detaches and reattaches disks serially anyways.
It is implemented in the following manner:

* Drain lists all the pods with attached volumes. It evicts very first stateful-pod and waits for its related entry in Node object's `.status.volumesAttached` to be removed by KCM. It does the same for all the stateful-pods.
* It waits for `PvDetachTimeout` (default 2 minutes) for a given pod's PVC to be removed, else moves forward.

### How does `maxEvictRetries` configuration work with `drainTimeout` configuration?

It is recommended to only set `MachineDrainTimeout`. It satisfies the related requirements. `MaxEvictRetries` is auto-calculated based on `MachineDrainTimeout`, if `maxEvictRetries` is not provided. Following will be the overall behavior of both configurations together:

* If `maxEvictRetries` isn't set and only `maxDrainTimeout` is set:
  * MCM auto calculates the `maxEvictRetries` based on the `drainTimeout`.
* If `drainTimeout` isn't set and only `maxEvictRetries` is set:
  * Default `drainTimeout` and user provided `maxEvictRetries` for each pod is considered.
* If both `maxEvictRetries` and `drainTimoeut` are set:
  * Then both will be respected.
* If none are set:
  * Defaults are respected.

### What are the different phases of a machine?

A phase of a `machine` can be identified with `Machine.Status.CurrentStatus.Phase`. Following are the possible phases of a `machine` object:

* `Pending`: Machine creation call has succeeded. MCM is waiting for machine to join the cluster.
* `CrashLoopBackOff`: Machine creation call has failed. MCM will retry the operation after a minor delay.
* `Running`: Machine creation call has succeeded. Machine has joined the cluster successfully and corresponding node doesn't have `node.gardener.cloud/critical-components-not-ready` taint.
* `Unknown`: Machine [health checks](#what-health-checks-are-performed-on-a-machine) are failing, eg `kubelet` has stopped posting the status.

* `Failed`: Machine health checks have failed for a prolonged time. Hence it is declared failed by `Machine` controller in a [rate limited fashion](#how-does-rate-limiting-replacement-of-machine-work-in-mcm-how-is-it-related-to-meltdown-protection). `Failed` machines get replaced immediately.  

* `Terminating`: Machine is being terminated. Terminating state is set immediately when the deletion is triggered for the `machine` object. It also includes time when it's being drained. 

`NOTE`: No phase means the machine is being created on the cloud-provider.

Below is a simple phase transition diagram:
![image](images/machine_phase_transition.png)


### What health checks are performed on a machine?

Health check performed on a machine are:
- Existense of corresponding node obj
- Status of certain user-configurable node conditions. 
  - These conditions can be specified using the flag `--node-conditions` for OOT MCM provider or can be specified per machine object.
  - The default user configurable node conditions can be found [here](https://github.com/gardener/machine-controller-manager/blob/91eec24516b8339767db5a40e82698f9fe0daacd/pkg/util/provider/app/options/options.go#L60)
- `True` status of `NodeReady` condition . This condition shows kubelet's status

If any of the above checks fails , the machine turns to `Unknown` phase.
### How does rate limiting replacement of machine work in MCM? How is it related to meltdown protection?

Currently MCM replaces only `1` `Unkown` machine at a time per machinedeployment. This means until the particular `Unknown` machine get terminated and its replacement joins, no other `Unknown` machine would be removed.

The above is achieved by enabling `Machine` controller to turn machine from `Unknown` -> `Failed` only if the above condition is met. `MachineSet` controller on the other hand marks `Failed` machine as `Terminating` immediately.

One reason for this rate limited replacement was to ensure that in case of network failures , where node's kubelet can't reach out to kube-apiserver , all nodes are not removed together i.e. `meltdown protection`.
In gardener context however, [DWD](https://github.com/gardener/dependency-watchdog/blob/master/docs/concepts/prober.md#origin) is deployed to deal with this scenario, but to stay protected from corner cases , this mechanism has been introduced in MCM.

`NOTE`: Rate limiting replacement is not yet configurable 
### How MCM responds when scale-out/scale-in is done during rolling update of a machinedeployment?
 
 `Machinedeployment` controller executes the logic of `scaling` BEFORE logic of `rollout`. It identifies `scaling` by comparing the `deployment.kubernetes.io/desired-replicas` of each machineset under the machinedeployment with machinedeployment's `.spec.replicas`. If the difference is found for any machineSet, a scaling event is detected.

 Case `scale-out` -> ONLY New machineSet is scaled out <br>
 Case `scale-in`  -> ALL machineSets(new or old) are scaled in , in proportion to their replica count , any leftover is adjusted in the largest machineSet.

During update for scaling event, a machineSet is updated if any of the below is true for it:
- `.spec.Replicas` needs update
- `deployment.kubernetes.io/desired-replicas` needs update

Once scaling is achieved, rollout continues.

### How does MCM prioritize the machines for deletion on scale-down of machinedeployment?
There could be many machines under a machinedeployment with different phases, creationTimestamp. When a scale down is triggered, MCM decides to remove the machine using the following logic:

* Machine with least value of `machinepriority.machine.sapcloud.io` annotation is picked up.
* If all machines have equal priorities, then following precedence is followed:
  * Terminating > Failed > CrashloopBackoff > Unknown > Pending > Available > Running
* If still there is no match, the machine with oldest creation time (.i.e. creationTimestamp) is picked up.

## How some unhealthy machines are drained quickly ?

If a node is unhealthy for more than the `machine-health-timeout` specified for the `machine-controller`, the controller
health-check moves the machine phase to `Failed`. By default, the `machine-health-timeout` is 10` minutes.

`Failed` machines have their deletion timestamp set and the machine then moves to the `Terminating` phase. The node
drain process is initiated. The drain process is invoked either *gracefully* or *forcefully*.

The usual drain process is graceful. Pods are evicted from the node and the drain process waits until any existing
attached volumes are mounted on new node. However, if the node `Ready` is `False` or the `ReadonlyFilesystem` is `True`
for greater than `5` minutes (non-configurable), then a forceful drain is initiated. In a forceful drain, pods are deleted
and `VolumeAttachment` objects associated with the old node are also marked for deletion. This is followed by the deletion of the
cloud provider VM associated with the `Machine` and then finally ending with the `Node` object deletion. 

During the deletion of the VM we only delete the local data disks and boot disks associated with the VM. The disks associated
with persistent volumes are left un-touched as their attach/de-detach, mount/unmount processes are handled by k8s
attach-detach controller in conjunction with the CSI driver.

# Troubleshooting
### My machine is stuck in deletion for 1 hr, why?

In most cases, the `Machine.Status.LastOperation` provides information around why a machine can't be deleted.
Though following could be the reasons but not limited to:

* Pod/s with misconfigured PDBs block the drain operation. PDBs with `maxUnavailable` set to 0, doesn't allow the eviction of the pods. Hence, drain/eviction is retried till `MachineDrainTimeout`. Default `MachineDrainTimeout` could be as large as ~2hours. Hence, blocking the machine deletion. 
  * Short term: User can manually delete the pod in the question, _with caution_. 
  * Long term: Please set more appropriate PDBs which allow disruption of at least one pod.
* Expired cloud credentials can block the deletion of the machine from infrastructure.
* Cloud provider can't delete the machine due to internal errors. Such situations are best debugged by using cloud provider specific CLI or cloud console.


### My machine is not joining the cluster, why?

In most cases, the `Machine.Status.LastOperation` provides information around why a machine can't be created.
It could possibly be debugged with following steps:

* Firstly make sure all the relevant controllers like `kube-controller-manager` , `cloud-controller-manager` are running.
* Verify if the machine is actually created in the cloud. User can use the `Machine.Spec.ProviderId` to query the machine in cloud.
* A Kubernetes node is generally bootstrapped with the cloud-config. Please verify, if `MachineDeployment` is pointing the correct `MachineClass`, and `MachineClass` is pointing to the correct `Secret`. The secret object contains the actual cloud-config in `base64` format which will be used to boot the machine.
* User must also check the logs of the MCM pod to understand any broken logical flow of reconciliation.

### My rolling update is stuck , why?

The following can be the reason:
- Insufficient capacity for the new instance type the machineClass mentions.
- [Old machines are stuck in deletion](#my-machine-is-stuck-in-deletion-for-1-hr-why)
- If you are using Gardener for setting up kubernetes cluster, then machine object won't turn to `Running` state until `node-critical-components` are ready. Refer [this](https://github.com/gardener/gardener/blob/master/docs/usage/node-readiness.md) for more details. 


# Developer

### How should I test my code before submitting a PR?

- Developer can locally setup the MCM using following [guide](https://github.com/gardener/machine-controller-manager/blob/master/docs/development/local_setup.md)
- Developer must also enhance the unit tests related to the incoming changes.
- Developer can locally run the unit test by executing:
```
make test-unit
```
- Developer can locally run [integration tests](development/integration_tests.md) to ensure basic functionality of MCM is not altered.

### I need to change the APIs, what are the recommended steps?

Developer should add/update the API fields at both of the following places:

* https://github.com/gardener/machine-controller-manager/blob/master/pkg/apis/machine/types.go
* https://github.com/gardener/machine-controller-manager/tree/master/pkg/apis/machine/v1alpha1

Once API changes are done, auto-generate the code using following command:
```
make generate
```
Please ignore the API-violation errors for now.

### How can I update the dependencies of MCM?

MCM uses `gomod` for depedency management.
Developer should add/udpate depedency in the go.mod file. Please run following command to automatically tidy the dependencies.
```
make tidy
```


# In the context of Gardener

### How can I configure MCM using Shoot resource?

All of the knobs of MCM can be configured by the `workers` [section](https://github.com/gardener/gardener/blob/master/example/90-shoot.yaml#L29-L126) of the shoot resource.

* Gardener creates a `MachineDeployment` per zone for each worker-pool under `workers` section. 
* `workers.dataVolumes` allows to attach multiple disks to a machine during creation. Refer the [link](https://github.com/gardener/gardener/blob/master/example/90-shoot.yaml#L29-L126).
* `workers.machineControllerManager` allows configuration of multiple knobs of the `MachineDeployment` from the shoot resource.

### How is my worker-pool spread across zones?

Shoot resource allows the worker-pool to spread across multiple zones using the field `workers.zones`. Refer [link](https://github.com/gardener/gardener/blob/master/example/90-shoot.yaml#L115).

* Gardener creates one `MachineDeployment` per zone. Each `MachineDeployment` is initiated with the following replica:
```
MachineDeployment.Spec.Replicas = (Workers.Minimum)/(Number of availibility zones)
```
