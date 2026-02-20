# Machine Preservation — Usage Guide

This document explains how to **use machine preservation** to retain machines and their backing VMs.

### What is preservation in MCM?

A machine and its backing node can be preserved by an end-user/SRE/operator to retain machines and their backing VMs for debugging, analysis, or operational safety. 

A preserved machine/node has the following properties:
- When a machine in `Failed` phase is preserved, it continues to stay in `Failed` state until `machinePreserveTimeout` runs out, without getting terminated. This allows end-users and SREs to debug the machine and backing node, and take necessary actions to recover the machine if needed.
- If a machine is in its `Failed` phase and is preserved, on recovering from failure, the machine can be moved to `Running` phase and the backing node can be uncordoned to allow scheduling of pods again.
- If a machine is preserved and is in its `Failed` phase, MCM drains the backing node of all pods, but the daemonset pods remain on the node.
- If a machine is preserved in its `Running` phase, the MCM adds the CA scale-down-disabled annotation to prevent the CA from scaling down the machine in case of underutilization.
- When the machineset is scaled down, machines in the machineset marked for preservation are de-prioritized for deletion.


> Note: If a user sets a deletion timestamp on the machine/node (e.g., by using kubectl delete machine/node), the machine and backing node will be deleted. Preservation will not prevent this.

> Note: If the desired replica count for a machineset cannot be met without scaling down preserved machines, the required number of preserved machines will be scaled-down.

### Changes in a machine/node object on preservation:

- If the machine is preserved in `Running` phase:
    - The `PreserveExpiryTime` is set in the machine's status to indicate when preservation will end.
    - The CA scale-down-disabled annotation is added.
    - The `NodeCondition` of `Type=Preserved` is updated to show that the preservation was successful.
    - If a machine has no backing node, only `PreserveExpiryTime` is set.
- If the machine is preserved and in `Failed` phase:
    - The `PreserveExpiryTime` is set in the machine's status to indicate when preservation will end.
    - The CA scale-down-disabled annotation is added.
    - The backing node is drained of all pods but the daemonset pods remain.
    - The `NodeCondition` of `Type=Preserved` is updated to show that the preservation was successful.
    - If a machine has no backing node, only `PreserveExpiryTime` is set.

### Changes in a machine/node object when preservation stops:
- If the machine is in `Running` phase:
    - The `PreserveExpiryTime` is cleared.
    - The CA scale-down-disabled annotation is removed.
    - The `NodeCondition` of `Type=Preserved` is updated to show that the preservation has stopped.
    - If a machine has no backing node, only `PreserveExpiryTime` is cleared.
- If the machine is in `Failed` phase:
    - The `PreserveExpiryTime` is cleared.
    - The CA scale-down-disabled annotation is removed.
    - The `NodeCondition` of `Type=Preserved` is updated to show that the preservation has stopped.
    - If a machine has no backing node, only `PreserveExpiryTime` is cleared.
    - The machine is moved to `Terminating` phase by MCM.

---
### How can a machine be preserved?

- The preservation feature offers two modes of preservation:
    - Manual preservation by adding annotations
    - Auto-preservation by MCM by specifying `AutoPreserveFailedMachineMax` for a workerpool. This value is distributed evenly across zones (MCD).
- For manual preservation, the end-user and operators must annotate either the node or the machine objects with annotation key: `node.machine.sapcloud.io/preserve` and the desired value, as described in the table below.
- If there is no backing node, the machine object can still be annotated.

#### Configuration (Shoot Spec)

Preservation is enabled and controlled per **worker pool**:

```yaml  
apiVersion: core.gardener.cloud/v1beta1  
kind: Shoot  
...  
spec:  
  workers:  
  - cri:      
	  name: containerd    
    name: worker1
    autoPreserveFailedMachineMax: 1
    machineControllerManager:      
	  machinePreserveTimeout: 72h    
    
```

#### Configuration Semantics
- `AutoPreserveFailedMachineMax` : Maximum number of failed machines that can be auto-preserved concurrently in a worker pool. This value is distributed across machineDeployments (zones) in the worker pool. If the limit is reached, additional failed machines will not be preserved and will proceed to termination as usual.
- `machinePreserveTimeout` : Duration after which preserved machines are automatically released

> Note: ⚠️ Changes to `machinePreserveTimeout` apply only to preservation done after the change.

### Preservation annotations

annotation key: `node.machine.sapcloud.io/preserve`

**Manual Annotation values:**

| Annotation value | Purpose                                                                     |
| ---------------- |-----------------------------------------------------------------------------|
| when-failed      | To be added when the machine/node needs to be preserved **only on failure** |
| now              | To be added when the machine/node needs to be preserved **now**             |
| false            | To be added if a machine should not be auto-preserved by MCM                |

**Auto-preservation Annotation values added by MCM:**

| Annotation value | Purpose                                                                                                                                               |
| ---------------- |-------------------------------------------------------------------------------------------------------------------------------------------------------|
| auto-preserve    | Added by MCM to indicate that a machine has been **auto-preserved** on failure. This machine will be counted towards **AutoPreserveFailedMachineMax** |

### ⚠️ Preservation Annotation semantics:

Both node and machine objects can be annotated for preservation. 
However, if both machine and node have the preservation annotation, the node's annotation value (even if set to "") is honoured and the machine's annotation is deleted. 
To prevent confusion and unintended behaviour, it is advised to use the feature by annotating only the node or the machine, and not both.

When the `PreserveExpiryTime` of a preserved machine is reached, the preservation will be stopped. Additionally, the preservation annotation is removed to prevent undesired re-preservation of the same machine. This is applicable for both manual and auto-preservation.

When a machine is annotated with value `false`, the annotation and value is not removed by MCM. This is to explicitly indicate that the machine should not be auto-preserved by MCM. If the annotation value is set to empty or the annotation is deleted, MCM will again auto-preserve the machine if it is in `Failed` phase and the `AutoPreserveFailedMachineMax` limit is not reached.

---
### How to manually stop preservation before PreserveExpiryTime:
**In the case of manual preservation:**
To manually stop preservation, the preservation annotation must be deleted from whichever object (node/machine) is annotated for preservation.

**In the case of auto-preservation:**
To stop auto-preservation, the machine should be annotated with `node.machine.sapcloud.io/preserve=false`. 
Deleting the annotation value or setting it to empty will not stop preservation since MCM will again auto-preserve the machine if it is in `Failed` phase and the `AutoPreserveFailedMachineMax` limit is not reached. 
Setting the annotation value to `false` explicitly indicates that the machine should not be auto-preserved by MCM anymore, and the preservation will be stopped.

---

### How to prevent a machine from being auto-preserved by MCM:

To prevent a `Failed` machine from being auto-preserved the node/machine object must be annotated with `node.machine.sapcloud.io/preserve=false`. If a currently preserved machine is annotated with `false`, the preservation will be stopped.
Here too, the preservation annotation semantics from above applies - if both machine and node are annotated, the node's annotation value is honoured and the machine's annotation is deleted.

If the preserve annotation value is set to empty or the annotation is deleted, MCM will again auto-preserve the machine if it is in `Failed` phase and the `AutoPreserveFailedMachineMax` limit is not reached.

---
### What happens when a machine recovers from failure and moves to `Running` during preservation?

Depending on the annotation value - (`now/when-failed/auto-preserve`), the behaviour differs. This is to reflect the meaning behind the annotation value.

1. `now`: on recovery from failure, machine preservation continues until `PreserveExpiryTime`
2. `when-failed`: on recovery from failure, machine preservation stops. This is because the annotation value clearly expresses that a machine must be preserved only when `Failed`. If the annotation is not changed, and the machine fails again, the machine is preserved again.
3. `auto-preserve`: since MCM performs auto-preservation of machines only when they are `Failed`, on recovery, the machine preservation is stopped.

In all the cases, when the machine moves to `Running` during preservation, the backing node is uncordoned to allow pods to be scheduled on it again.

>Note: When a machine recovers to `Running` and preservation is stopped, CA's `scale-down-unneeded-time` comes into play. If the node's utilization is below the utilization threshold configured even after `scale-down-unneeded-time`, CA will scale down the machine.

---
### Important Notes & Limitations
- Rolling updates: Preservation is ignored; Failed machines are replaced as usual.
- Shoot hibernation overrides preservation.
- Replica enforcement: Preserved machines count towards MachineDeployment replicas, and can be scaled down if the desired replica count is reduced below the number of preserved machines.
- Scale-down preference: Preserved machines are the last to be scaled down.
- Preservation status is visible via Node Conditions and Machine Status fields.
- machinePreserveTimeout changes do not affect existing preserved machines. Operators may edit PreserveExpiryTime directly if required to extend preservation.
