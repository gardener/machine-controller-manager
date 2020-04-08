# Excess Reserve Capacity

<!-- TOC -->

- [Excess Reserve Capacity](#excess-reserve-capacity)
    - [Goal](#goal)
    - [Note](#note)
    - [Possible Approaches](#possible-approaches)
        - [Approach 1: Enhance Machine-controller-manager to also entertain the excess machines](#approach-1-enhance-machine-controller-manager-to-also-entertain-the-excess-machines)
        - [Approach 2: Enhance Cluster-autoscaler by simulating fake pods in it](#approach-2-enhance-cluster-autoscaler-by-simulating-fake-pods-in-it)
        - [Approach 3: Enhance cluster-autoscaler to support pluggable scaling-events](#approach-3-enhance-cluster-autoscaler-to-support-pluggable-scaling-events)
        - [Approach 4: Make intelligent use of Low-priority pods](#approach-4-make-intelligent-use-of-low-priority-pods)

<!-- /TOC -->

## Goal

Currently, autoscaler optimizes the number of machines for a given application-workload. Along with effective resource utilization, this feature brings concern where, many times, when new application instances are created - they don't find space in existing cluster. This leads the cluster-autoscaler to create new machines via MachineDeployment, which can take from 3-4 minutes to ~10 minutes, for the machine to really come-up and join the cluster. In turn, application-instances have to wait till new machines join the cluster.

One of the promising solutions to this issue is Excess Reserve Capacity. Idea is to keep a certain number of machines or percent of resources[cpu/memory] always available, so that new workload, in general, can be scheduled immediately unless huge spike in the workload. Also, the user should be given enough flexibility to choose how many resources or how many machines should be kept alive and non-utilized as this affects the Cost directly.

## Note

- We decided to go with Approach-4 which is based on low priority pods. Please find more details here: https://github.com/gardener/gardener/issues/254
- Approach-3 looks more promising in long term, we may decide to adopt that in future based on developments/contributions in autoscaler-community. 

## Possible Approaches

Following are the possible approaches, we could think of so far.

### Approach 1: Enhance Machine-controller-manager to also entertain the excess machines

- Machine-controller-manager currently takes care of the machines in the shoot cluster starting from creation-deletion-health check to efficient rolling-update of the machines. From the architecture point of view, MachineSet makes sure that X number of machines are always **running and healthy**. MachineDeployment controller smartly uses this facility to perform rolling-updates.

- We can expand the scope of MachineDeployment controller to maintain excess number of machines by introducing new parallel independent controller named *MachineTaint* controller. This will result in MCM to include Machine, MachineSet, MachineDeployment, MachineSafety, MachineTaint controllers. MachineTaint controller does not need to introduce any new CRD - analogy fits where taint-controller also resides into kube-controller-manager.

- Only Job of MachineTaint controller will be:
  - List all the Machines under each MachineDeployment.
  - Maintain taints of [*noSchedule* and *noExecute*](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/) on `X` latest MachineObjects.
  - There should be an event-based informer mechanism where MachineTaintController gets to know about any Update/Delete/Create event of MachineObjects - in turn, maintains the *noSchedule* and *noExecute* taints on all the *latest* machines.
        - Why latest machines?
            - Whenever autoscaler decides to add new machines - essentially ScaleUp event - taints from the older machines are removed and newer machines get the taints. This way X number of Machines immediately becomes free for new pods to be scheduled.
            - While ScaleDown event, autoscaler specifically mentions which machines should be deleted, and that should not bring any concerns. Though we will have to put proper label/annotation defined by autoscaler on taintedMachines, so that autoscaler does not consider the taintedMachines for deletion while scale-down.
                * Annotation on tainted node: `"cluster-autoscaler.kubernetes.io/scale-down-disabled": "true"`
- Implementation Details:
  - Expect new **optional field** *ExcessReplicas* in `MachineDeployment.Spec`. MachineDeployment controller now adds both `Spec.Replicas` and `Spec.ExcessReplicas`[if provided], and considers that as a standard desiredReplicas.
        - Current working of MCM will not be affected if ExcessReplicas field is kept nil.
  - MachineController currently reads the *NodeObject* and sets the MachineConditions in MachineObject. Machine-controller will now also read the taints/labels from the MachineObject - and maintains it on the *NodeObject*.

- We expect cluster-autoscaler to intelligently make use of the provided feature from MCM.
  - CA gets the input of *min:max:excess* from Gardener. CA continues to set the `MachineDeployment.Spec.Replicas` as usual based on the application-workload.
  - In addition, CA also sets the `MachieDeployment.Spec.ExcessReplicas` .
  - Corner-case:
        * CA should decrement the excessReplicas field accordingly when *desiredReplicas+excessReplicas* on MachineDeployment goes beyond *max*.

### Approach 2: Enhance Cluster-autoscaler by simulating fake pods in it

- There was already an attempt by community to support this feature.
    - Refer for details to: https://github.com/kubernetes/autoscaler/pull/77/files

### Approach 3: Enhance cluster-autoscaler to support pluggable scaling-events

- Forked version of cluster-autoscaler could be improved to plug-in the algorithm for excess-reserve capacity.
- Needs further discussion around upstream support.
- Create golang channel to separate the algorithms to trigger scaling (hard-coded in cluster-autoscaler, currently) from the algorithms about how to to achieve the scaling (already pluggable in cluster-autoscaler). This kind of separation can help us introduce/plug-in new algorithms (such as based node resource utilisation) without affecting existing code-base too much while almost completely re-using the code-base for the actual scaling.
- Also this approach is not specific to our fork of cluster-autoscaler. It can be made upstream eventually as well.

### Approach 4: Make intelligent use of Low-priority pods

- Refer to: [pod-priority-preemption](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/)
- TL; DR:
  - High priority pods can preempt the low-priority pods which are already scheduled.
  - Pre-create bunch[equivivalent of X shoot-control-planes] of low-priority pods with priority of zero, then start creating the workload pods with better priority which will reschedule the low-priority pods or otherwise keep them in pending state if the limit for max-machines has reached.
  - This is still alpha feature.
