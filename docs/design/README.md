# Design of Node Controller Manager

The design of the Node Controller Manager is influenced by the Kube Controller Manager, where-in multiple sub-controllers are used to manage the Kubernetes clients.

## Design Principles

It's designed to run in the master plane of a Kubernetes cluster. It follows the best principles and practices of writing controllers, including, but not limited to:

- Reusing code from kube-controller-manager
- leader election to allow HA deployments of the controller
- `workqueues` and multiple thread-workers
- `SharedInformers` that limit to minimum network calls, de-serialization and provide helpful create/update/delete events for resources
- rate-limiting to allow back-off in case of network outages and general instability of other cluster components
- sending events to respected resources for easy debugging and overview
- Prometheus metrics, health and (optional) profiling endpoints

## Objects of Node Controller Manager

Node Controller Manager makes use of 4 CRD objects and 1 Kubernetes secret object to manage machines. They are as follows,
1. Instance-class: A template that contains cloud provider specific details used to create instances.
1. Instance: Node Controller Manager's object that actually represents a VM.
1. Instance-set: A group of instances managed by the Node Controller Manager. 
1. Instance-deploy: A group of instance-sets managed by the Node Controller Manager to allow updating instances.
1. Secret: A kubernetes secret that stores cloudconfig (initialization scripts used to create VMs) and cloud specific credentials

## Components of Node Controller Manager

Node Controller Manager is made up of 3 sub-controllers as of now. They are -
1. Instance Controller: Used to create/update/delete instances. It is the only controller which actually talks to the cloud providers.
1. Instance Set Controller: Used to manage instance-sets. This controller makes sure that desired number of instances are always up and running healthy.
1. Instance Deployment Controller: Used to update instances from one version to another by manipulating the instance-set objects. 

All these controllers work in an co-operative manner. They form a parent-child relationship with Instance Deployment Controller being the grandparent, Instance Set Controller being the parent, and Instance Controller being the child. 

## Future Plans
The following is a short list of future plans,
1. **Integrate the cluster-autoscaler** to act upon instance-deployment objects, used to manage the required number of instances based on the load of the cluster.
2. **Support other cloud providers** like Azure, GCP, OpenStack to name a few.
3. Integrate a garbage collector to terminate any orphan VMs.
4. Build a comprehensive testing framework.
5. Fix bugs that exist in the current implementation.

## Limitations
The following are the list of limitations,
1. It currently only supports AWS, but will support a larger set of cloud providers in future.
2. This component is brand new and hence not yet production-grade
