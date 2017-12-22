# node-controller-manager

The Node Controller Manager bundles multiple Kubernetes controllers, that help with the creation, maintance and operation of different instance (node) Kubernetes resources. 

It's designed to run in master plane of a kubernetes cluster and follows the best principles and practices of writing controllers, including, but not limited to:

- leader election to allow HA deployments of the controller
- `workqueues` and multiple thread-workers
- `SharedInformers` that limit to minimum network calls, deserialization and provide helpful create/update/delete events for resources
- rate-limiting to allow back-off in case of network outages and general instability of other cluster components
- enabling / disabling bundled controllers
- sending events to respected resources for easy debugging and overview
- Prometheus metrics, health and (optional) profiling endpoints

## Build

```bash 
go build -i cmd/node-controller-manager/controller_manager.go
```

## Running

For testing/development purposes, you can create your own kubernetes cluster and download its kubeconfig file. Use this kubeconfig file to be passed as an argument to your node-controller-manager. You can run the node-controller-manager on your local machine as mentioned below

```bash
go run cmd/node-controller-manager/controller_manager.go --kubeconfig=$KUBECONFIG --v=2
```

$KUBECONFIG: Refers to the location to your kubeconfig file

## Deploy

To deploy your node-controller-manager you would have to create an docker image out of the compiled binary and finally deploy this docker image on your kubernetes cluster. Exact steps to deploy are yet to be finalized on.
