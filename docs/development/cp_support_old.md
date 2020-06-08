# Adding support for a new cloud provider

For adding support for a new cloud provider in the Machine Controller Manager, follow the steps described below. Replace provider with your provider-name.

1. Add a ProviderMachineClass CRD similar to existing AWSMachineClass into `kubernetes/crds.yaml`.
1. Add ProviderMachineClass structs similar to existing AWSMachineClass into the machine APIs into `pkg/apis/machine/types.go` and `pkg/apis/machine/v1alpha1/types.go`. This would be the machineClass template used to describe provider specific configurations.
1. Add the Go structures of your machine class (list) to `pkg/apis/machine/register.go` and `pkg/apis/machine/v1alpha1/register.go` to allow reporting events on these objects.
1. Regenerate the machine API clients by running `./hack/generate-code`
1. Add validation for the new provider machine class at `pkg/apis/machine/validation/providermachineclass.go` similar to `pkg/apis/machine/validation/awsmachineclass.go`
1. Update `pkg/controller/machine_util.go` to allow validation of the new provider.
1. Add a new driver into `pkg/driver/driver_provider.go` similar to `pkg/driver/driver_aws.go` to implement the driver interface.
1. Update `pkg/driver/driver.go` to add a new switch case to support the new provider driver.
1. Add a new method in `pkg/controller/machine_safety.go` called checkProviderMachineClass similar to the existing method called checkAWSMachineClass present in the same file. Now invoke this method as a go-routine in the method checkVMObjects.
1. Extend the `StartControllers()` function in `cmd/machine-controller-manager/app/controllermanager.go` to only start if your new machine class is under the available resources.
1. Update `pkg/controller/controller.go` to add new providerMachineClassLister, providerMachineClassQueue, awsMachineClassSynced into the controller struct. Also initialize them in NewController() method.
1. Add a new file `pkg/controller/providermachineclass.go` that allows re-queuing of machines which refer to an modified providerMachineClass.
1. Update `pkg/controller/controller.go` to extend `WaitForCacheSync` and `.Shutdown()` similar to other cloud providers.
1. Update the example ClusterRole in `kubernetes/deployment/in-tree/clusterrole.yaml` to allow operations on your new machine class.
1. Update `pkg/controller/controller.go`, `pkg/controller/secret.go`, `pkg/controller/secret_util.go` to add event handlers to add/remove finalizers referenced by your machine Class. Refer [this commit](https://github.com/gardener/machine-controller-manager/pull/104/commits/013f70726b1057aed1cf7fe0f0449922ab9a256a).
