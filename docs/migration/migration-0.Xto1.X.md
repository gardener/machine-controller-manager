# Migration Plan: MCM 0.X to 1.X
* Machine controller manager will soon be released with new major version 1.0.0. This version will contain breaking machine-api changes. This document aims to provide steps to migrate from MCM-0.X to new major version MCM-1.X without restarting/disrupting machines of existing kubernetes-cluster powered by MCM.

* Steps below are tested with only strict-sequence below, it may lead to unexpected behaviour otherwise.

* Summary: Essentially the migration replaces the old-machine-api objects by newer machine-api objects, and starts the new major version of MCM to interpret the same. The major API change is related to introduction of generic MachineClass against provider specific MachineClasses[e.g AWSMachineClass]. 
    * We will use the adoption logic of MCM, where new Machine objects can be adopted by MachineSet and eventually by MachineDeployment.
    * Steps below assumes AWS as cloud-provider for ease of understanding, but it should work any cloud-provider supported by MCM.

## Steps:
1.  Switch off the old version of MCM-0.X .
    - `kubectl scale deployment --replicas=0 machine-controller-manager -n NAMESPACE`

1. Get the local copy of following machine-api objects: Machine, MachineSet, MachineDeployment, AWSMachineClass(applicable cloud)
    - `kubectl get machinedeployment -n NAMESPACE -o yaml > backup-machinedeployment.yaml`
    - `kubectl get machineset -n NAMESPACE -o yaml > backup-machineset.yaml`
    - `kubectl get machine -n NAMESPACE -o yaml > backup-machine.yaml`
    - `kubectl get awsmachineclass -n NAMESPACE -o yaml > backup-awsmachineclass.yaml`

1. Delete all the machine-api specific objects. Given MCM is not running, it should not affect the real kubernetes machines. We will have to remove the finalizers from the all objects manually as MCM is not running at the moment [else objects will not be deleted].
    - `kubectl delete machinedeployment/machineset/machine/awsmachineclass -n NAMESPACE --all`
    - `kubectl edit machinedeployment/machineset/machine/awsmachineclass -n NAMESPACE`
        - remove the 2 lines of `finalizers` from each object.

1. Apply the new custom resource definitions. This CRD includes new definition of generic machineclass of kind "MachineClass".
- `kubectl apply -f kubernetes/crds.yaml`

1. Prepare the `new-machine.yaml` taking inputs from `backup-machine.yaml`
    - Basically we have to copy the `Status.Node` and `Spec.ProviderID` from backup to new machine files. This will help MCM to co-relate the node-objects with machine-objects.
    - We also need to update the `Spec.Class.Kind` from `AWSMachineClass` to `MachineClass` with appropriate name at `Spec.Class.Name`.
    - This exercise should be done for all the machine object.
    - Please check following backup and new machine file (here)[link] for reference.

1. Prepare the `new-machineclass.yaml` from `backup-awsmachineclass`file. We have to basically use the `ProviderSpec` blob in generic machineclass.
    - Please check following backup and new machineclass file (here)[link] for reference.

1. Start the MCM 1.X along with the provider specific side-car driver-container in MCM pod.
    - `kubectl apply -f machine-controller-manager-1.X.yaml -n NAMESPACE`

1. Apply the `new-machineclass.yaml` and `new-machine.yaml` to kubernetes cluster so that new machine-api-objects can be registered. 
    - `kubectl apply -f new-machine.yaml/new-machineclass.yaml -n NAMESPACE`
    - If everything is fine, any machines should not be created or deleted at this point in time.

1. Prepare `new-machineset.yaml` and `new-machinedeployment.yaml` from corresponding backup files. Please make sure to have appropriate `Spec.Selector.MatchLabels` in new files to cover existing MachineObjects.
    - Update the `Spec.Template.Spec.Class.Kind` from `AWSMachineClass` to `MachineClass` with appropriate name as well.
    - Please check following backup and new files (here)[link] for reference.
    - If everything is fine, this step should not create or delete any machines.

1. Verify if MachineDeployment, MachineSet and MachineObjects have been properly adopted by checking the Status and OwnerReference[in MachineSet/Machine] fields in those objects.
    - `kubectl get machinedeployment/machineset/machine -o yaml -n NAMESPACE`
