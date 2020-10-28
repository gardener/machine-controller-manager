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

* [Internals](#internals)
  * [What is the high level design of MCM?](#What-is-the-high-level-design-of-MCM)
  * [What are the different configuration options in MCM?](#What-are-the-different-configuration-options-in-MCM)
  * [What are the different timeouts/configurations in a machine's lifecycle?](#What-are-the-different-timeouts/configurations-in-a-machine's-lifecycle)
  * [How is the drain of a machine implemented?](#How-is-the-drain-of-a-machine-implemented)
  * [How are the stateful applications drained during machine deletion??](#How-are-the-stateful-applications-drained-during-machine-deletion?)
  * [How does maxEvictRetries configuration work with drainTimeout configuration?](#How-does-maxEvictRetries-configuration-work-with-drainTimeout-configuration)
  * [What are the different phases of a machine?](#What-are-the-different-phases-of-a-machine)

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

### What is Machine Controller Manager ?

Machine Controller Manager aka MCM is a bunch of controllers used for the lifecycle management of the worker machines. It reconciles a set of CRDs such as `Machine`, `MachineSet`, `MachineDeployment` which depicts the functionality of `Pod`, `Replicaset`, `Deployment` of the core Kubernetes respectively. Read more about it at [README](https://github.com/gardener/machine-controller-manager/tree/master/docs).

* Gardener uses MCM to manage its Kubernetes nodes of the shoot cluster. However, by design, MCM can be used independent of Gardener.

### Why is my machine deleted ?

A machine is deleted by MCM generally for 2 reasons-

- Machine is unhealthy for at least `MachineHealthTimeout` period. The default `MachineHealthTimeout` is 10 minutes.

   * By default, a machine is considered unhealthy if any of the following node conditions - `DiskPressure`, `KernelDeadlock`, `FileSystem`, `Readonly` is set to `true`, or `KubeletReady` is set to `false`. However, this is something that is configurable using the following [flag](https://github.com/gardener/machine-controller-manager/blob/rel-v0.34.0/kubernetes/deployment/out-of-tree/deployment.yaml#L30).

- Machine is scaled down by the `MachineDeployment` resource.

   * This is very usual when an external controller cluster-autoscaler (aka CA) is used with MCM. CA deletes the under-utilized machines by scaling down the `MachineDeployment`. Read more about cluster-autoscaler's scale down behavior [here](https://github.com/gardener/autoscaler/blob/machine-controller-manager-provider/cluster-autoscaler/FAQ.md#how-does-scale-down-work).

### What are the different sub-controllers in MCM ?

MCM mainly contains the following sub-controllers:

* `MachineDeployment Controller`: Responsible for reconciling the `MachineDeployment` objects. It manages the lifecycle of the `MachineSet` objects.
* `MachineSet Controller`: Responsible for reconciling the `MachineSet` objects. It manages the lifecycle of the `Machine` objects.
* `Machine Controller`: responsible for reconciling the `Machine` objects. It manages the lifecycle of the actual VMs/machines created in cloud/on-prem. This controller has been moved out of tree. Please refer an AWS machine controller for more info - [link](https://github.com/gardener/machine-controller-manager-provider-gcp).
* Safety-controller: Responsible for handling the unidentified/unknown behaviors from the cloud providers. Please read more about its functionality [below](#what-is-safety-controller).

### What is Safety Controller in MCM ?

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

* Apply all the CRDs from [here](https://github.com/gardener/machine-controller-manager/tree/master/kubernetes/deployment/in-tree)
* Apply all the deployment, role-related objects from [here](https://github.com/gardener/machine-controller-manager/tree/master/kubernetes/deployment/in-tree).

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

A machine can be force deleted by adding the label `force-deletion: "True"` on the `machine` object if it's already being deleted. During force deletion, MCM skips the drain function and simply triggers the deletion of the machine. This label should be used with caution as it can violate the PDBs for pods running on the machine.


# Internals
### What is the high level design of MCM?

Please refer the following [document](https://github.com/gardener/machine-controller-manager/tree/master/docs/design).

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
* Stateful applications (with PVCs) are serially evicted. Please find more info in this [answer below](How-are-the-stateful-applications-drained-during-machine-deletion?).


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
* `Running`: Machine creation call has succeeded. Machine has joined the cluster successfully.
* `Unknown`: Machine health checks are failing, eg `kubelet` has stopped posting the status.
* `Failed`: Machine health checks have failed for a prolonged time. Hence it is declared failed. `MachineSet` controller will replace such machines immediately.
* `Terminating`: Machine is being terminated. Terminating state is set immediately when the deletion is triggered for the `machine` object. It also includes time when it's being drained. 

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

* Verify if the machine is actually created in the cloud. User can use the `Machine.Spec.ProviderId` to query the machine in cloud.
* A Kubernetes node is generally bootstrapped with the cloud-config. Please verify, if `MachineDeployment` is pointing the correct `MachineClass`, and `MachineClass` is pointing to the correct `Secret`. The secret object contains the actual cloud-config in `base64` format which will be used to boot the machine.
* User must also check the logs of the MCM pod to understand any broken logical flow of reconciliation.


# Developer

### How should I test my code before submitting a PR?

- Developer can locally setup the MCM using following [guide](https://github.com/gardener/machine-controller-manager/blob/master/docs/development/local_setup.md)
- Developer must also enhance the unit tests related to the incoming changes.
- Developer can locally run the unit test by executing:
```
make test-unit
```

### I need to change the APIs, what are the recommended steps?

Developer should add/update the API fields at both of the following places:

* https://github.com/gardener/machine-controller-manager/blob/master/pkg/apis/machine/types.go
* https://github.com/gardener/machine-controller-manager/tree/master/pkg/apis/machine/v1alpha1

Once API changes are done, auto-generate the code using following command:
```
./hack/generate-code
```
Please ignore the API-violation errors for now.

### How can I update the dependencies of MCM?

MCM uses `gomod` for depedency management.
Developer should add/udpate depedency in the go.mod file. Please run following command to automatically revendor the dependencies.
```
make revendor
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
