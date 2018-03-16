# Design of Machine Controller Manager

The design of the Machine Controller Manager is influenced by the Kube Controller Manager, where-in multiple sub-controllers are used to manage the Kubernetes clients.

## Design Principles

It's designed to run in the master plane of a Kubernetes cluster. It follows the best principles and practices of writing controllers, including, but not limited to:

- Reusing code from kube-controller-manager
- leader election to allow HA deployments of the controller
- `workqueues` and multiple thread-workers
- `SharedInformers` that limit to minimum network calls, de-serialization and provide helpful create/update/delete events for resources
- rate-limiting to allow back-off in case of network outages and general instability of other cluster components
- sending events to respected resources for easy debugging and overview
- Prometheus metrics, health and (optional) profiling endpoints

## Objects of Machine Controller Manager

Machine Controller Manager makes use of 4 CRD objects and 1 Kubernetes secret object to manage machines. They are as follows,
1. Machine-class: Represents a template that contains cloud provider specific details used to create machines.
1. Machine: Represents a VM which is backed by the cloud provider.
1. Machine-set: Represents a group of machines managed by the Machine Controller Manager.
1. Machine-deployment: Represents a group of machine-sets managed by the Machine Controller Manager to allow updating machines.
1. Secret: Represents a kubernetes secret that stores cloudconfig (initialization scripts used to create VMs) and cloud specific credentials

## Components of Machine Controller Manager

Machine Controller Manager is made up of 3 sub-controllers as of now. They are -
1. Machine Controller: Used to create/update/delete machines. It is the only controller which actually talks to the cloud providers.
1. Machine Set Controller: Used to manage machine-sets. This controller makes sure that desired number of machines are always up and running healthy.
1. Machine Deployment Controller: Used to update machines from one version to another by manipulating the machine-set objects.
1. Machine Safety Controller: A safety net controller that terminates orphan VMs and freezes machineSet/machineDeployment objects which are overshooting or timing out while trying to join nodes to the cluster.

All these controllers work in an co-operative manner. They form a parent-child relationship with Machine Deployment Controller being the grandparent, Machine Set Controller being the parent, and Machine Controller being the child.

## Future Plans
The following is a short list of future plans,
1. **Integrate the cluster-autoscaler** to act upon machine-deployment objects, used to manage the required number of machines based on the load of the cluster.
2. **Support other cloud providers** like OpenStack.
3. Integrate a garbage collector to terminate any orphan VMs.
4. Build a comprehensive testing framework.
5. Fix bugs that exist in the current implementation.

### Todos Doc
[This link](https://docs.google.com/document/d/10ruoL6VLVOEG2htYluY5T0-qvijXkw0Do_qE0tmexos/edit?usp=sharing) contains the working doc for the todos which are planned in the near future.

## Limitations
The following are the list of limitations,
1. It currently only supports AWS, Azure and GCP, but will support a larger set of cloud providers in future.
2. This component is brand new and hence not yet production-grade
