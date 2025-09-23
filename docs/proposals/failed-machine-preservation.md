# Preservation of Failed Machines

<!-- TOC -->

- [Preservation of Failed Machines](#preservation-of-failed-machines)
    - [Objective](#objective)
    - [Solution Design](#solution-design)
    - [State Machine](#state-machine)
    - [Use Cases](#use-cases)
        

<!-- /TOC -->

## Objective

Currently, the Machine Controller Manager(MCM) moves Machines with errors to the `Unknown` phase, and after the configured `machineHealthTimeout` seconds, to the `Failed` phase.
`Failed` machines are swiftly moved to the `Terminating` phase during which the node is drained and the `Machine` object is deleted. This rapid cleanup prevents SRE/operators/support from conducting an analysis on the VM and makes finding root cause of failure more difficult.

This document proposes enhancing MCM, such that:
* VMs of `Failed` machines are retained temporarily for analysis
* There is a configurable limit to the number of `Failed` machines that can be preserved
* There is a configurable limit to the duration for which such machines are preserved
* Users can specify which healthy machines they would like to preserve in case of failure 
* Users can request MCM to delete a preserved `Failed` machine, even before the timeout expires

## Solution Design

In order to achieve the objectives mentioned, the following are proposed:
1. Enhance `machineControllerManager` configuration in the `ShootSpec`, to specify the max number of failed machines to be preserved,
and the time duration for which these machines will be preserved.
    ```
    machineControllerManager:
       failedMachinePreserveMax: 2
       failedMachinePreserveTimeout: 3h
    ```
    * Since gardener worker pool can correspond to `1..N` MachineDeployments depending on number of zones, `failedMachinePreserveMax` will be distributed across N machine deployments.
    * `failedMachinePreserveMax` must be chosen such that it can be appropriately distributed across the MachineDeployments.
2. Allow user/operator to explicitly request for preservation of a machine if it moves to `Failed` phase with the use of an annotation : `node.machine.sapcloud.io/preserve-when-failed=true`.
When such an annotated machine transitions from `Unknown` to `Failed`, it is prevented from moving to `Terminating` phase until  `failedMachinePreserveTimeout` expires. 
   * A user/operator can request MCM to stop preserving a preserved `Failed` machine by adding/modifying the annotation: `node.machine.sapcloud.io/preserve-when-failed=false`. 
   * For a machine thus annotated, MCM will move it to `Terminating` phase even if `failedMachinePreserveTimeout` has not expired.
3. If an un-annotated machine moves to `Failed` phase, and the `failedMachinePreserveMax` has not been reached, MCM will auto-preserve this machine.
4. MCM will be modified to introduce a new stage in the `Failed` phase: `machineutils.PreserveFailed`, and a failed machine that is preserved by MCM will be transitioned to this stage after moving to `Failed`. 
   * In this new stage, pods can be evicted and scheduled on other healthy machines, and the user/operator can wait for the corresponding VM to potentially recover. If the machine moves to `Running` phase on recovery, new pods can be scheduled on it. It is yet to be determined whether this feature will be required.
5. Machines of a MachineDeployment in `PreserveFailed` stage will also be counted towards the replica count and the enforcement of maximum machines allowed for the MachineDeployment.


## State Machine

The behaviour described above can be summarised using the state machine below:

```
(Running Machine)
├── [User adds `node.machine.sapcloud.io/preserve-when-failed=true`] → (Running + Requested)
└── [Machine fails + capacity available] → (PreserveFailed)

(Running + Requested)
├── [Machine fails + capacity available] → (PreserveFailed)
├── [Machine fails + no capacity] → Failed → Terminating 
└── [User removes `node.machine.sapcloud.io/preserve-when-failed=true`] → (Running)

(PreserveFailed)
├── [User adds `node.machine.sapcloud.io/preserve-when-failed=false`] → Terminating
└── [failedMachinePreserveTimeout expires] → Terminating

```
In the above state machine, the phase `Running` also includes machines that are in the process of creation for which no errors have been encountered yet.
The transition of moving a machine from `PreserveFailed` to `Running` has not been shown since we haven't determined whether it is in scope for the current iteration of this feature.

## Use Cases:

### Use Case 1: Proactive Preservation Request
**Scenario:** Operator suspects a machine might fail and wants to ensure preservation for analysis.
#### Steps:
1. Operator annotates node with `node.machine.sapcloud.io/preserve-when-failed=true`
2. Machine fails later
3. MCM preserves the machine (if capacity allows)
4. Operator analyzes the failed VM
5. Operator releases the failed machine by setting `node.machine.sapcloud.io/preserve-when-failed=false` on the node object

### Use Case 2: Automatic Preservation
**Scenario:** Machine fails unexpectedly, no prior annotation.
#### Steps:
1. Machine transitions to Failed state
2. MCM checks preservation capacity
3. If capacity available, machine moved to `PreserveFailed` phase by MCM
4. After timeout, machine is terminated by MCM

### Use Case 3: Capacity Management
**Scenario:** Multiple machines fail when preservation capacity is full.
#### Steps:
1. Machines M1, M2 already preserved (capacity = 2)
2. Machine M3 fails with annotation `node.machine.sapcloud.io/preserve-when-failed=true` set
3. MCM cannot preserve M3 due to capacity limits
4. M3 moved from `Failed` to `Terminating` by MCM, following which it is deleted

### Use Case 4: Early Release
**Scenario:** Operator has performed his analysis and no longer requires machine to be preserved

#### Steps:
1. Machine M1 is in `PreserveFailed` phase
2. Operator adds: `node.machine.sapcloud.io/preserve-when-failed=false` to node.
3. MCM transitions M1 to `Terminating`
4. Capacity becomes available for preserving future `Failed` machines.

## Limitations

1. During rolling updates we will NOT honor preserving Machines. The Machine will be replaced with a healthy one if it moves to Failed phase.
2. Since gardener worker pool can correspond to 1..N MachineDeployments depending on number of zones, we will need to distribute the `failedMachinePreserveMax` across N machine deployments.
So, even if there are no failed machines preserved in other zones, the max per zone would still be enforced. Hence, the value of `failedMachinePreserveMax` should be chosen appropriately. 
