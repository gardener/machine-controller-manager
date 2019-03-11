# Frequently Asked Questions

The answers in this FAQ apply to the newest (HEAD) version of Machine Controller Manager. If
you're using an older version of MCM please refer to corresponding version of
this document:

# Older versions

* [Machine Controller Manager 0.X](https://github.com/gardener/machine-controller-manager/tree/master/docs/FAQ.md) 


# Table of Contents:
<!--- TOC BEGIN -->
* [Basics](#basics)
  * [What is Machine Controller Manager?](#what-is-machine-controller-manager)
* [How to?](#how-to)
* [Internals](#internals)
* [Troubleshooting](#troubleshooting)
* [Developer](#developer)
  * [How can I run tests?](#how-can-i-run-e2e-tests)
  * [How should I test my code before submitting PR?](#how-should-i-test-my-code-before-submitting-pr)
  * [How can I update MCM dependencies?](#how-can-i-update-mcm-dependencies-particularly-k8siokubernetes)
<!--- TOC END -->

# Basics

### What is Machine Controller Manager ?
Machine controller manager is a bunch of controllers to declaratively manage the nodes for a kubernetes cluster. MCM is also the core component of project Gardener, though it can be used indepedently as well.

### What are different controllers in MCM ?
MCM contains controllers - MachineDeployment, MachineSet, Machine, and Safety-controller.
These controllers are analogous to Deployment, ReplicaSet and Pod.

### What is Safety-controller ?
Safety controller was built later, learning from experiences of running mcm over many cloud-providers. Safety-controller contains handlers which deletes the orphan machines/resources if found. It also freezes the core-mcm functionality if there are more machine-objects than expected in control-plane.

# How to?

### Is there any depedency for installing MCM in already running kuberentes cluster ?
With `reasonably` new version of kubernetes,there are no dependecies for installing MCM. You can already start following the [steps](https://github.com/gardener/machine-controller-manager/blob/master/docs/deployment/kubernetes.md#deploying-the-machine-controller-manager-into-a-kubernetes-cluster)
* Tl;DR 

```
# Please fill up the necessary place-holders in the yaml files below.
kubectl apply -f kubernetes/crds.yaml
kubectl apply -f kubernetes/deployment/clusterrole.yaml
kubectl apply -f kubernetes/deployment/clusterrolebinding.yaml
kubectl apply -f kubernetes/deployment/deployment.yaml
```

Enjoy creating the machines then ..

```
# Please fill up the necessary place-holders in the yaml files below.
kubectl apply -f kubernetes/machine_classes/aws-machine-class.yaml
kubectl apply -f kubernetes/Secrets/aws-secret.yaml
kubectl apply -f kubernetes/machine_objects/machine.yaml
```
