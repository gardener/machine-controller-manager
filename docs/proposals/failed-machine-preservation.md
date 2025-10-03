# Preservation of Failed Machines

<!-- TOC -->

- [Preservation of Failed Machines](#preservation-of-failed-machines)
    - [Objective](#objective)
    - [Proposal](#proposal)
    - [Use Cases](#use-cases)

<!-- /TOC -->

## Objective

Currently, the Machine Controller Manager(MCM) moves Machines with errors to the `Unknown` phase, and after the configured `machineHealthTimeout`, to the `Failed` phase.
`Failed` machines are swiftly moved to the `Terminating` phase during which the node is drained and the `Machine` object is deleted. This rapid cleanup prevents SRE/operators/support from conducting an analysis on the VM and makes finding root cause of failure more difficult.

Moreover, in cases where a node seems healthy but all the workload on it are facing issues, there is a need for operators to be able to cordon/drain the node and conduct their analysis without the cluster-autoscaler scaling down the node.

This document proposes enhancing MCM, such that:
* VMs of machines are retained temporarily for analysis
* There is a configurable limit to the number of machines that can be preserved
* There is a configurable limit to the duration for which such machines are preserved
* Users can specify which healthy machines they would like to preserve in case of failure 
* Users can request MCM to release a preserved machine, even before the timeout expires, so that MCM can transition the machine to either `Running` or `Terminating` phase, as the case may be.

## Proposal

In order to achieve the objectives mentioned, the following are proposed:
1. Enhance `machineControllerManager` configuration in the `ShootSpec`, to specify the max number of machines to be preserved,
and the time duration for which these machines will be preserved.
    ```
    machineControllerManager:
       machinePreserveMax: 1
       machinePreserveTimeout: 72h
    ```
    * This configuration will be set per worker pool.
    * Since gardener worker pool can correspond to `1..N` MachineDeployments depending on number of zones, `machinePreserveMax` will be distributed across N machine deployments.
    * `machinePreserveMax` must be chosen such that it can be appropriately distributed across the MachineDeployments.
    * Example: if `machinePreserveMax` is set to 2, and the worker pool has 2 zones, then the maximum number of machines that will be preserved per zone is 1.
2. MCM will be modified to include a new phase `Preserved` to indicate that the machine has been preserved by MCM.
3. Allow user/operator to request for preservation of a specific machine/node with the use of annotations : `node.machine.sapcloud.io/preserve=now` and `node.machine.sapcloud.io/preserve=when-failed`.
4. When annotation `node.machine.sapcloud.io/preserve=now` is added to a `Running` machine, the following will take place:
   - `cluster-autoscaler.kubernetes.io/scale-down-disabled: "true"` is added to the node to prevent CA from scaling it down.
   - `machine.CurrentStatus.PreserveExpiryTime` is updated by MCM as $machine.CurrentStatus.PreserveExpiryTime = currentTime+machinePreserveTimeout$
   - The machine stage is changed to `Preserved`
   - After timeout, the `node.machine.sapcloud.io/preserve=now` and `cluster-autoscaler.kubernetes.io/scale-down-disabled: "true"` are deleted, the machine phase is changed to `Running` and the CA may delete the node. The `machine.CurrentStatus.PreserveExpiryTime` is set to `nil`.
   - Number of machines explicitly annotated will count towards enforcing `machinePreserveMax`. On breach, the annotation will be rejected.
5. When annotation `node.machine.sapcloud.io/preserve=when-failed` is added to a `Running` machine and the machine goes to `Failed`, the following will take place:
   - The machine phase is changed to `Preserved`.
   - Pods (other than daemonset pods) are drained.
   - `machine.CurrentStatus.PreserveExpiryTime` is updated by MCM as $machine.CurrentStatus.PreserveExpiryTime = currentTime+machinePreserveTimeout$
   - After timeout, the `node.machine.sapcloud.io/preserve=when-failed` is deleted. The phase is changed to `Terminating`.
   - Number of machines explicitly annotated will count towards enforcing `machinePreserveMax`. On breach, the annotation will be rejected.
6. When an un-annotated machine goes to `Failed` phase and the $count(machinesAnnotatedForPreservation)+count(AutoPreservedMachines)<machinePreserveMax$
   - The machine's phase is changed to `Preserved`.
   - Pods (other than DaemonSet pods) are drained.
   - `machine.CurrentStatus.PreserveExpiryTime` is updated by MCM as $machine.CurrentStatus.PreserveExpiryTime = currentTime+machinePreserveTimeout$
   - After timeout, the phase is changed to `Terminating`.
   - Number of machines in `Preserved` phase count towards enforcing `machinePreserveMax`.
   - In the rest of the doc, the preservation of such un-annotated failed machines is referred to as **"auto-preservation"**.
7. If a `Failed` machine is currently in `Preserved` and after timeout its VM/node is found to be Healthy, the machine will be moved to `Running`.
8. A user/operator can request MCM to stop preserving a machine/node in `Preserved` stage using the annotation: `node.machine.sapcloud.io/preserve=false`. 
   * For a machine thus annotated, MCM will move it either to `Running` phase or `Terminating` depending on the phase of the machine before it was moved to `Preserved`.
9. Machines of a MachineDeployment in `Preserved` stage will also be counted towards the replica count and in the enforcement of maximum machines allowed for the MachineDeployment.
10. At any point in time $count(machinesAnnotatedForPreservation)+count(PreservedMachines)<=machinePreserveMax$. 

## Use Cases:

### Use Case 1: Proactive Preservation Request
**Scenario:** Operator suspects a machine might fail and wants to ensure preservation for analysis.
#### Steps:
1. Operator annotates node with `node.machine.sapcloud.io/preserve=when-failed`, provided `machinePreserveMax` is not violated
2. Machine fails later
3. MCM preserves the machine
4. Operator analyzes the failed VM

### Use Case 2: Automatic Preservation
**Scenario:** Machine fails unexpectedly, no prior annotation.
#### Steps:
1. Machine transitions to `Failed` phase
2. If `machinePreserveMax` is not breached, machine moved to `Preserved` phase by MCM
3. After `machinePreserveTimeout`, machine is terminated by MCM

### Use Case 3: Preservation Request for Analysing Running Machine
**Scenario:** Workload on machine failing. Operator wishes to diagnose.
#### Steps:
1. Operator annotates node with `node.machine.sapcloud.io/preserve=now`, provided `machinePreserveMax` is not violated
2. MCM preserves machine and prevents CA from scaling it down
3. Operator analyzes the machine

### Use Case 4: Early Release
**Scenario:** Operator has performed his analysis and no longer requires machine to be preserved
#### Steps:
1. Machine is in `Preserved` phase
2. Operator adds: `node.machine.sapcloud.io/preserve=false` to node.
3. MCM transitions machine to `Running` or `Terminating`, depending on which phase it was in before moving to `Preserved`, even though `machinePreserveTimeout` has not expired
4. Capacity becomes available for preserving future annotated machines or for auto-preservation of `Failed` machines.


## Limitations

1. During rolling updates we will NOT honor preserving Machines. The Machine will be replaced with a healthy one if it moves to Failed phase.
2. Since gardener worker pool can correspond to 1..N MachineDeployments depending on number of zones, we will need to distribute the `machinePreserveMax` across N machine deployments.
So, even if there are no failed machines preserved in other zones, the max per zone would still be enforced. Hence, the value of `machinePreserveMax` should be chosen appropriately. 
