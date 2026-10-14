# API Reference

## Packages
- [machine.sapcloud.io/v1alpha1](#machinesapcloudiov1alpha1)


## machine.sapcloud.io/v1alpha1

Package v1alpha1 is the v1alpha1 version of the API.
Package v1alpha1 is a version of the API.

### Resource Types
- [Machine](#machine)
- [MachineClass](#machineclass)
- [MachineDeployment](#machinedeployment)
- [MachineSet](#machineset)



#### ClassSpec



ClassSpec is the class specification of machine



_Appears in:_
- [MachineSetSpec](#machinesetspec)
- [MachineSpec](#machinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiGroup` _string_ | API group to which it belongs |  |  |
| `kind` _string_ | Kind for machine class |  |  |
| `name` _string_ | Name of machine class |  |  |


#### ConditionStatus

_Underlying type:_ _string_

ConditionStatus are valid condition statuses



_Appears in:_
- [MachineDeploymentCondition](#machinedeploymentcondition)
- [MachineSetCondition](#machinesetcondition)

| Field | Description |
| --- | --- |
| `True` |  |
| `False` |  |
| `Unknown` |  |


#### CurrentStatus



CurrentStatus contains information about the current status of Machine.



_Appears in:_
- [MachineStatus](#machinestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[MachinePhase](#machinephase)_ |  |  |  |
| `timeoutActive` _boolean_ |  |  |  |
| `lastUpdateTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | Last update time of current status |  |  |
| `preserveExpiryTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | PreserveExpiryTime is the time at which MCM will stop preserving the machine |  |  |


#### InPlaceUpdateMachineDeployment



InPlaceUpdateMachineDeployment specifies the spec to control the desired behavior of inplace update.



_Appears in:_
- [MachineDeploymentStrategy](#machinedeploymentstrategy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxUnavailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#intorstring-intstr-util)_ | The maximum number of machines that can be unavailable during the update.<br />Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).<br />Absolute number is calculated from percentage by rounding down.<br />This can not be 0 if MaxSurge is 0.<br />Example: when this is set to 30%, the old machine set can be scaled down to 70% of desired machines<br />immediately when the update starts. Once new machines are ready, old machine set<br />can be scaled down further, followed by scaling up the new machine set, ensuring<br />that the total number of machines available at all times during the update is at<br />least 70% of desired machines. |  | Optional: \{\} <br /> |
| `maxSurge` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#intorstring-intstr-util)_ | The maximum number of machines that can be scheduled above the desired number of<br />machines.<br />Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).<br />This can not be 0 if MaxUnavailable is 0.<br />Absolute number is calculated from percentage by rounding up.<br />Example: when this is set to 30%, the new machine set can be scaled up immediately when<br />the update starts, such that the total number of old and new machines does not exceed<br />130% of desired machines. Once old machines have been killed,<br />new machine set can be scaled up further, ensuring that total number of machines running<br />at any time during the update is utmost 130% of desired machines. |  | Optional: \{\} <br /> |
| `orchestrationType` _[OrchestrationType](#orchestrationtype)_ | OrchestrationType specifies the orchestration type for the inplace update. |  |  |


#### LastOperation



LastOperation suggests the last operation performed on the object



_Appears in:_
- [MachineSetStatus](#machinesetstatus)
- [MachineStatus](#machinestatus)
- [MachineSummary](#machinesummary)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `description` _string_ | Description of the current operation |  |  |
| `errorCode` _string_ | ErrorCode of the current operation if any |  | Optional: \{\} <br /> |
| `lastUpdateTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | Last update time of current operation |  |  |
| `state` _[MachineState](#machinestate)_ | State of operation |  |  |
| `type` _[MachineOperationType](#machineoperationtype)_ | Type of operation |  |  |


#### Machine



Machine is the representation of a physical or virtual machine.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `machine.sapcloud.io/v1alpha1` | | |
| `kind` _string_ | `Machine` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[MachineSpec](#machinespec)_ | Spec contains the specification of the machine |  |  |
| `status` _[MachineStatus](#machinestatus)_ | Status contains fields depicting the status |  |  |


#### MachineClass



MachineClass can be used to templatize and re-use provider configuration
across multiple Machines / MachineSets / MachineDeployments.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `machine.sapcloud.io/v1alpha1` | | |
| `kind` _string_ | `MachineClass` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `nodeTemplate` _[NodeTemplate](#nodetemplate)_ | NodeTemplate contains subfields to track all node resources and other node info required to scale nodegroup from zero |  | XPreserveUnknownFields: \{\} <br />Optional: \{\} <br /> |
| `credentialsSecretRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#secretreference-v1-core)_ | CredentialsSecretRef can optionally store the credentials (in this case the SecretRef does not need to store them).<br />This might be useful if multiple machine classes with the same credentials but different user-datas are used. |  |  |
| `providerSpec` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#rawextension-runtime-pkg)_ | Provider-specific configuration to use during node creation. |  | XPreserveUnknownFields: \{\} <br /> |
| `provider` _string_ | Provider is the combination of name and location of cloud-specific drivers. |  |  |
| `secretRef` _[SecretReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#secretreference-v1-core)_ | SecretRef stores the necessary secrets such as credentials or userdata. |  |  |


#### MachineConfiguration



MachineConfiguration describes the configurations useful for the machine-controller.



_Appears in:_
- [MachineSpec](#machinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `drainTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#duration-v1-meta)_ | MachineDraintimeout is the timeout after which machine is forcefully deleted. |  | Optional: \{\} <br /> |
| `healthTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#duration-v1-meta)_ | MachineHealthTimeout is the timeout after which machine is declared unhealhty/failed. |  | Optional: \{\} <br /> |
| `creationTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#duration-v1-meta)_ | MachineCreationTimeout is the timeout after which machinie creation is declared failed. |  | Optional: \{\} <br /> |
| `inPlaceUpdateTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#duration-v1-meta)_ | MachineInPlaceUpdateTimeout is the timeout after which in-place update is declared failed. |  | Optional: \{\} <br /> |
| `machinePreserveTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#duration-v1-meta)_ | MachinePreserveTimeout is the timeout after which the machine preservation is stopped |  | Optional: \{\} <br /> |
| `disableHealthTimeout` _boolean_ | DisableHealthTimeout if set to true, health timeout will be ignored. Leading to machine never being declared failed.<br />This is intended to be used only for in-place updates. |  | Optional: \{\} <br /> |
| `maxEvictRetries` _integer_ | MaxEvictRetries is the number of retries that will be attempted while draining the node. |  | Optional: \{\} <br /> |
| `nodeConditions` _string_ | NodeConditions are the set of conditions if set to true for MachineHealthTimeOut, machine will be declared failed. |  | Optional: \{\} <br /> |


#### MachineDeployment



MachineDeployment enables declarative updates for machines and MachineSets.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `machine.sapcloud.io/v1alpha1` | | |
| `kind` _string_ | `MachineDeployment` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[MachineDeploymentSpec](#machinedeploymentspec)_ | Specification of the desired behavior of the MachineDeployment. |  | Optional: \{\} <br /> |
| `status` _[MachineDeploymentStatus](#machinedeploymentstatus)_ | Most recently observed status of the MachineDeployment. |  | Optional: \{\} <br /> |


#### MachineDeploymentCondition



MachineDeploymentCondition describes the state of a MachineDeployment at a certain point.



_Appears in:_
- [MachineDeploymentStatus](#machinedeploymentstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[MachineDeploymentConditionType](#machinedeploymentconditiontype)_ | Type of MachineDeployment condition. |  |  |
| `status` _[ConditionStatus](#conditionstatus)_ | Status of the condition, one of True, False, Unknown. |  |  |
| `lastUpdateTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | The last time this condition was updated. |  |  |
| `lastTransitionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | Last time the condition transitioned from one status to another. |  |  |
| `reason` _string_ | The reason for the condition's last transition. |  |  |
| `message` _string_ | A human readable message indicating details about the transition. |  |  |


#### MachineDeploymentConditionType

_Underlying type:_ _string_

MachineDeploymentConditionType are valid conditions of MachineDeployments



_Appears in:_
- [MachineDeploymentCondition](#machinedeploymentcondition)

| Field | Description |
| --- | --- |
| `Available` | Available means the MachineDeployment is available, ie. at least the minimum available<br />replicas required are up and running for at least minReadySeconds.<br /> |
| `Progressing` | Progressing means the MachineDeployment is progressing. Progress for a MachineDeployment is<br />considered when a new machine set is created or adopted, and when new machines scale<br />up or old machines scale down. Progress is not estimated for paused MachineDeployments. It is also updated<br />if progressDeadlineSeconds is not specified(treated as infinite deadline), in which case it would never be updated to "false".<br /> |
| `ReplicaFailure` | ReplicaFailure is added in a MachineDeployment when one of its machines fails to be created<br />or deleted.<br /> |
| `Frozen` | MachineDeploymentFrozen is added in a MachineDeployment when one of its<br />machineSet has either the "freeze" label or the "Frozen" condition on it.<br /> |


#### MachineDeploymentSpec



MachineDeploymentSpec is the specification of the desired behavior of the MachineDeployment.



_Appears in:_
- [MachineDeployment](#machinedeployment)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ | Number of desired machines. This is a pointer to distinguish between explicit<br />zero and not specified. Defaults to 0. |  | Optional: \{\} <br /> |
| `selector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#labelselector-v1-meta)_ | Label selector for machines. Existing MachineSets whose machines are<br />selected by this will be the ones affected by this MachineDeployment. |  | Optional: \{\} <br /> |
| `template` _[MachineTemplateSpec](#machinetemplatespec)_ | Template describes the machines that will be created. |  |  |
| `strategy` _[MachineDeploymentStrategy](#machinedeploymentstrategy)_ | The MachineDeployment strategy to use to replace existing machines with new ones. |  | Optional: \{\} <br /> |
| `minReadySeconds` _integer_ | Minimum number of seconds for which a newly created machine should be ready<br />without any of its container crashing, for it to be considered available.<br />Defaults to 0 (machine will be considered available as soon as it is ready) |  | Optional: \{\} <br /> |
| `revisionHistoryLimit` _integer_ | The number of old MachineSets to retain to allow rollback.<br />This is a pointer to distinguish between explicit zero and not specified. |  | Optional: \{\} <br /> |
| `paused` _boolean_ | Indicates that the MachineDeployment is paused and will not be processed by the<br />MachineDeployment controller. |  | Optional: \{\} <br /> |
| `rollbackTo` _[RollbackConfig](#rollbackconfig)_ | DEPRECATED.<br />The config this MachineDeployment is rolling back to. Will be cleared after rollback is done. |  | Optional: \{\} <br /> |
| `progressDeadlineSeconds` _integer_ | The maximum time in seconds for a MachineDeployment to make progress before it<br />is considered to be failed. The MachineDeployment controller will continue to<br />process failed MachineDeployments and a condition with a ProgressDeadlineExceeded<br />reason will be surfaced in the MachineDeployment status. Note that progress will<br />not be estimated during the time a MachineDeployment is paused. This is not set<br />by default, which is treated as infinite deadline. |  | Optional: \{\} <br /> |
| `autoPreserveFailedMachineMax` _integer_ | The maximum number of failed machines in the machine deployment that can be auto-preserved.<br />In the gardener context, this number is derived from the AutoPreserveFailedMachineMax set at the worker level, distributed amongst the worker's machine deployments |  | Optional: \{\} <br /> |


#### MachineDeploymentStatus



MachineDeploymentStatus is the most recently observed status of the MachineDeployment.



_Appears in:_
- [MachineDeployment](#machinedeployment)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | The generation observed by the MachineDeployment controller. |  | Optional: \{\} <br /> |
| `replicas` _integer_ | Total number of non-terminated machines targeted by this MachineDeployment (their labels match the selector). |  | Optional: \{\} <br /> |
| `updatedReplicas` _integer_ | Total number of non-terminated machines targeted by this MachineDeployment that have the desired template spec. |  | Optional: \{\} <br /> |
| `readyReplicas` _integer_ | Total number of ready machines targeted by this MachineDeployment. |  | Optional: \{\} <br /> |
| `availableReplicas` _integer_ | Total number of available machines (ready for at least minReadySeconds) targeted by this MachineDeployment. |  | Optional: \{\} <br /> |
| `preservedFailedReplicas` _integer_ | PreservedFailedReplicas is the number of preserved machines in Failed phase targeted by this MachineDeployment |  | Optional: \{\} <br /> |
| `unavailableReplicas` _integer_ | Total number of unavailable machines targeted by this MachineDeployment. This is the total number of<br />machines that are still required for the MachineDeployment to have 100% available capacity. They may<br />either be machines that are running but not yet available, machines that still have not been created, or<br />machines that are preserved in Failed phase. |  | Optional: \{\} <br /> |
| `conditions` _[MachineDeploymentCondition](#machinedeploymentcondition) array_ | Represents the latest available observations of a MachineDeployment's current state. |  |  |
| `collisionCount` _integer_ | Count of hash collisions for the MachineDeployment. The MachineDeployment controller uses this<br />field as a collision avoidance mechanism when it needs to create the name for the<br />newest MachineSet. |  | Optional: \{\} <br /> |
| `failedMachines` _[MachineSummary](#machinesummary) array_ | FailedMachines has summary of machines on which lastOperation Failed |  | Optional: \{\} <br /> |


#### MachineDeploymentStrategy



MachineDeploymentStrategy describes how to replace existing machines with new ones.



_Appears in:_
- [MachineDeploymentSpec](#machinedeploymentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[MachineDeploymentStrategyType](#machinedeploymentstrategytype)_ | Type of MachineDeployment. Can be "Recreate" or "RollingUpdate". Default is RollingUpdate. |  | Optional: \{\} <br /> |
| `rollingUpdate` _[RollingUpdateMachineDeployment](#rollingupdatemachinedeployment)_ | Rolling update config params. Present only if MachineDeploymentStrategyType =<br />RollingUpdate. |  | Optional: \{\} <br /> |
| `inPlaceUpdate` _[InPlaceUpdateMachineDeployment](#inplaceupdatemachinedeployment)_ | InPlaceUpdate update config params. Present only if MachineDeploymentStrategyType =<br />InPlaceUpdate. |  | Optional: \{\} <br /> |


#### MachineDeploymentStrategyType

_Underlying type:_ _string_

MachineDeploymentStrategyType are valid strategy types for rolling MachineDeployments



_Appears in:_
- [MachineDeploymentStrategy](#machinedeploymentstrategy)

| Field | Description |
| --- | --- |
| `Recreate` | RecreateMachineDeploymentStrategyType means that all existing machines will be killed before creating new ones.<br /> |
| `RollingUpdate` | RollingUpdateMachineDeploymentStrategyType means that old MCs will be replaced by new one using rolling update i.e gradually scale down the old MCs and scale up the new one.<br /> |
| `InPlaceUpdate` | InPlaceUpdateMachineDeploymentStrategyType signifies that VMs will be updated in place and<br />machine objects will gradually transition from the old MachineSet to the new MachineSet without requiring VM recreation.<br /> |


#### MachineOperationType

_Underlying type:_ _string_

MachineOperationType is a label for the operation performed on a machine object.



_Appears in:_
- [LastOperation](#lastoperation)

| Field | Description |
| --- | --- |
| `Create` | MachineOperationCreate indicates that the operation was a create<br /> |
| `Update` | MachineOperationUpdate indicates that the operation was an update<br /> |
| `InPlaceUpdate` | MachineOperationInPlaceUpdate indicates that the operation was an in-place update<br /> |
| `DrainNode` | MachineOperationDrainNode indicates that the operation was a drain node<br /> |
| `HealthCheck` | MachineOperationHealthCheck indicates that the operation was a health check of node object<br /> |
| `Delete` | MachineOperationDelete indicates that the operation was a delete<br /> |


#### MachinePhase

_Underlying type:_ _string_

MachinePhase is a label for the condition of a machine at the current time.



_Appears in:_
- [CurrentStatus](#currentstatus)

| Field | Description |
| --- | --- |
| `Pending` | MachinePending means that the machine is being created<br /> |
| `Available` | MachineAvailable means that machine is present on provider but hasn't joined cluster yet<br /> |
| `Running` | MachineRunning means node is ready and running successfully<br /> |
| `Terminating` | MachineTerminating means node is terminating<br /> |
| `Unknown` | MachineUnknown indicates that the node is not ready at the movement<br /> |
| `Failed` | MachineFailed means operation timed out<br /> |
| `CrashLoopBackOff` | MachineCrashLoopBackOff means machine creation is failing. It means that machine object is present but there is no corresponding VM.<br /> |
| `InPlaceUpdating` | MachineInPlaceUpdating means machine is being updated in-place<br /> |
| `InPlaceUpdateSuccessful` | MachineInPlaceUpdateSuccessful means machine in-place update was successful<br /> |
| `InPlaceUpdateFailed` | MachineInPlaceUpdateFailed means machine in-place update failed<br /> |


#### MachineSet



MachineSet TODO





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `machine.sapcloud.io/v1alpha1` | | |
| `kind` _string_ | `MachineSet` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[MachineSetSpec](#machinesetspec)_ |  |  | Optional: \{\} <br /> |
| `status` _[MachineSetStatus](#machinesetstatus)_ |  |  | Optional: \{\} <br /> |


#### MachineSetCondition



MachineSetCondition describes the state of a machine set at a certain point.



_Appears in:_
- [MachineSetStatus](#machinesetstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[MachineSetConditionType](#machinesetconditiontype)_ | Type of machine set condition. |  |  |
| `status` _[ConditionStatus](#conditionstatus)_ | Status of the condition, one of True, False, Unknown. |  |  |
| `lastTransitionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | The last time the condition transitioned from one status to another. |  | Optional: \{\} <br /> |
| `reason` _string_ | The reason for the condition's last transition. |  | Optional: \{\} <br /> |
| `message` _string_ | A human readable message indicating details about the transition. |  | Optional: \{\} <br /> |


#### MachineSetConditionType

_Underlying type:_ _string_

MachineSetConditionType is the condition on machineset object



_Appears in:_
- [MachineSetCondition](#machinesetcondition)

| Field | Description |
| --- | --- |
| `ReplicaFailure` | MachineSetReplicaFailure is added in a machine set when one of its machines fails to be created<br />due to insufficient quota, limit ranges, machine security policy, node selectors, etc. or deleted<br />due to kubelet being down or finalizers are failing.<br /> |
| `Frozen` | MachineSetFrozen is set when the machineset has exceeded its replica threshold at the safety controller<br /> |


#### MachineSetSpec



MachineSetSpec is the specification of a MachineSet.



_Appears in:_
- [MachineSet](#machineset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ |  |  | Optional: \{\} <br /> |
| `selector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#labelselector-v1-meta)_ |  |  | Optional: \{\} <br /> |
| `machineClass` _[ClassSpec](#classspec)_ |  |  | Optional: \{\} <br /> |
| `template` _[MachineTemplateSpec](#machinetemplatespec)_ |  |  | Optional: \{\} <br /> |
| `minReadySeconds` _integer_ |  |  | Optional: \{\} <br /> |
| `autoPreserveFailedMachineMax` _integer_ |  |  | Optional: \{\} <br /> |


#### MachineSetStatus



MachineSetStatus holds the most recently observed status of MachineSet.



_Appears in:_
- [MachineSet](#machineset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ | Replicas is the number of actual replicas. |  |  |
| `fullyLabeledReplicas` _integer_ | The number of pods that have labels matching the labels of the pod template of the replicaset. |  | Optional: \{\} <br /> |
| `readyReplicas` _integer_ | The number of ready replicas for this replica set. |  | Optional: \{\} <br /> |
| `availableReplicas` _integer_ | The number of available replicas (ready for at least minReadySeconds) for this replica set. |  | Optional: \{\} <br /> |
| `preservedFailedReplicas` _integer_ | PreservedFailedReplicas is the number of preserved replicas in Failed phase for this replica set |  |  |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent generation observed by the controller. |  | Optional: \{\} <br /> |
| `machineSetCondition` _[MachineSetCondition](#machinesetcondition) array_ | Represents the latest available observations of a replica set's current state. |  | Optional: \{\} <br /> |
| `lastOperation` _[LastOperation](#lastoperation)_ | LastOperation performed |  |  |
| `failedMachines` _[MachineSummary](#machinesummary)_ | FailedMachines has summary of machines on which lastOperation Failed |  | Optional: \{\} <br /> |
| `autoPreserveFailedMachineCount` _integer_ | AutoPreserveFailedMachineCount has a count of the number of failed machines in the machineset that are currently auto-preserved |  | Optional: \{\} <br /> |


#### MachineSpec



MachineSpec is the specification of a Machine.



_Appears in:_
- [Machine](#machine)
- [MachineTemplateSpec](#machinetemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `class` _[ClassSpec](#classspec)_ | Class contains the machineclass attributes of a machine |  | Optional: \{\} <br /> |
| `providerID` _string_ | ProviderID represents the provider's unique ID given to a machine |  | Optional: \{\} <br /> |
| `nodeTemplate` _[NodeTemplateSpec](#nodetemplatespec)_ | NodeTemplateSpec describes the data a node should have when created from a template |  | Optional: \{\} <br /> |


#### MachineState

_Underlying type:_ _string_

MachineState is a current state of the operation.



_Appears in:_
- [LastOperation](#lastoperation)

| Field | Description |
| --- | --- |
| `Processing` | MachineStateProcessing means operation is not yet complete<br /> |
| `Failed` | MachineStateFailed means operation failed<br /> |
| `Successful` | MachineStateSuccessful means operation completed successfully<br /> |


#### MachineStatus



MachineStatus holds the most recently observed status of Machine.



_Appears in:_
- [Machine](#machine)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `addresses` _[NodeAddress](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#nodeaddress-v1-core) array_ | Addresses of this machines. This field is only present if the MCM provider runs without a target cluster and may<br />be used by clients to determine how to connect to the machine, instead of the `Node.status.addresses` field. |  | Optional: \{\} <br /> |
| `conditions` _[NodeCondition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#nodecondition-v1-core) array_ | Conditions of this machine, same as node |  |  |
| `lastOperation` _[LastOperation](#lastoperation)_ | Last operation refers to the status of the last operation performed |  |  |
| `currentStatus` _[CurrentStatus](#currentstatus)_ | Current status of the machine object |  |  |
| `lastKnownState` _string_ | LastKnownState can store details of the last known state of the VM by the plugins.<br />It can be used by future operation calls to determine current infrastucture state |  | Optional: \{\} <br /> |


#### MachineSummary



MachineSummary store the summary of machine.



_Appears in:_
- [MachineDeploymentStatus](#machinedeploymentstatus)
- [MachineSetStatus](#machinesetstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the machine object |  |  |
| `providerID` _string_ | ProviderID represents the provider's unique ID given to a machine |  |  |
| `lastOperation` _[LastOperation](#lastoperation)_ | Last operation refers to the status of the last operation performed |  |  |
| `ownerRef` _string_ | OwnerRef |  |  |


#### MachineTemplateSpec



MachineTemplateSpec describes the data a machine should have when created from a template



_Appears in:_
- [MachineDeploymentSpec](#machinedeploymentspec)
- [MachineSetSpec](#machinesetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | XPreserveUnknownFields: \{\} <br />Optional: \{\} <br /> |
| `spec` _[MachineSpec](#machinespec)_ | Specification of the desired behavior of the machine.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### NodeTemplate



NodeTemplate contains subfields to track all node resources and other node info required to scale nodegroup from zero



_Appears in:_
- [MachineClass](#machineclass)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `instanceType` _string_ | Instance type of the node belonging to nodeGroup |  |  |
| `region` _string_ | Region of the expected node belonging to nodeGroup |  |  |
| `zone` _string_ | Zone of the expected node belonging to nodeGroup |  |  |
| `architecture` _string_ | CPU Architecture of the node belonging to nodeGroup |  | Optional: \{\} <br /> |


#### NodeTemplateSpec



NodeTemplateSpec describes the data a node should have when created from a template



_Appears in:_
- [MachineSpec](#machinespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | XPreserveUnknownFields: \{\} <br />Optional: \{\} <br /> |
| `spec` _[NodeSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#nodespec-v1-core)_ | NodeSpec describes the attributes that a node is created with. |  | Optional: \{\} <br /> |


#### OrchestrationType

_Underlying type:_ _string_

OrchestrationType specifies the orchestration type for the inplace update.



_Appears in:_
- [InPlaceUpdateMachineDeployment](#inplaceupdatemachinedeployment)

| Field | Description |
| --- | --- |
| `Auto` | OrchestrationTypeAuto signifies that the machines are automatically selected for update based on UpdateConfiguration.<br /> |
| `Manual` | OrchestrationTypeManual signifies that the user has to select the machines to be updated manually.<br /> |


#### RollbackConfig



RollbackConfig is the config to rollback a MachineDeployment



_Appears in:_
- [MachineDeploymentSpec](#machinedeploymentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `revision` _integer_ | The revision to rollback to. If set to 0, rollback to the last revision. |  | Optional: \{\} <br /> |


#### RollingUpdateMachineDeployment



RollingUpdateMachineDeployment is the spec to control the desired behavior of rolling update.



_Appears in:_
- [MachineDeploymentStrategy](#machinedeploymentstrategy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxUnavailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#intorstring-intstr-util)_ | The maximum number of machines that can be unavailable during the update.<br />Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).<br />Absolute number is calculated from percentage by rounding down.<br />This can not be 0 if MaxSurge is 0.<br />Example: when this is set to 30%, the old machine set can be scaled down to 70% of desired machines<br />immediately when the update starts. Once new machines are ready, old machine set<br />can be scaled down further, followed by scaling up the new machine set, ensuring<br />that the total number of machines available at all times during the update is at<br />least 70% of desired machines. |  | Optional: \{\} <br /> |
| `maxSurge` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#intorstring-intstr-util)_ | The maximum number of machines that can be scheduled above the desired number of<br />machines.<br />Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).<br />This can not be 0 if MaxUnavailable is 0.<br />Absolute number is calculated from percentage by rounding up.<br />Example: when this is set to 30%, the new machine set can be scaled up immediately when<br />the update starts, such that the total number of old and new machines does not exceed<br />130% of desired machines. Once old machines have been killed,<br />new machine set can be scaled up further, ensuring that total number of machines running<br />at any time during the update is utmost 130% of desired machines. |  | Optional: \{\} <br /> |


#### UpdateConfiguration



UpdateConfiguration specifies the udpate configuration for the deployment strategy.



_Appears in:_
- [InPlaceUpdateMachineDeployment](#inplaceupdatemachinedeployment)
- [RollingUpdateMachineDeployment](#rollingupdatemachinedeployment)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxUnavailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#intorstring-intstr-util)_ | The maximum number of machines that can be unavailable during the update.<br />Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).<br />Absolute number is calculated from percentage by rounding down.<br />This can not be 0 if MaxSurge is 0.<br />Example: when this is set to 30%, the old machine set can be scaled down to 70% of desired machines<br />immediately when the update starts. Once new machines are ready, old machine set<br />can be scaled down further, followed by scaling up the new machine set, ensuring<br />that the total number of machines available at all times during the update is at<br />least 70% of desired machines. |  | Optional: \{\} <br /> |
| `maxSurge` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#intorstring-intstr-util)_ | The maximum number of machines that can be scheduled above the desired number of<br />machines.<br />Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).<br />This can not be 0 if MaxUnavailable is 0.<br />Absolute number is calculated from percentage by rounding up.<br />Example: when this is set to 30%, the new machine set can be scaled up immediately when<br />the update starts, such that the total number of old and new machines does not exceed<br />130% of desired machines. Once old machines have been killed,<br />new machine set can be scaled up further, ensuring that total number of machines running<br />at any time during the update is utmost 130% of desired machines. |  | Optional: \{\} <br /> |


