# Adding support for a new cloud provider
For adding support for a new cloud provider in the Node Controller Manager, follow the steps described below. Replace provider with your provider-name.

1. Add a ProviderMachineClass CRD similar to existing AWSMachineClass into `kubernetes/crds.yaml`.
1. Add ProviderMachineClass structs similar to existing AWSMachineClass into the machine APIs into `pkg/apis/machine/types.go` and `pkg/apis/machine/v1alpha1/types.go`. This would be the machineClass template used to describe provider specific configurations.
1. Regenerate the machine API clients by running `make generate-files`
1. Add validation for the new provider machine class at `pkg/apis/machine/validation/providermachineclass.go` similar to `pkg/apis/machine/validation/awsmachineclass.go` 
1. Update `pkg/controller/machine_utils.go` to allow validation of the new provider.
1. Add a new driver into `pkg/driver/driver_provider.go` similar to `pkg/driver/driver_aws.go` to implement the driver interface. 
1. Update `pkg/driver/driver.go` to add a new switch case to support the new provider driver.
1. Update `pkg/controller/controller.go` to add new providerMachineClassLister, providerMachineClassQueue, awsMachineClassSynced into the controller struct. Also initialize them in NewController() method. 
1. Finally add a new file `pkg/controller/providermachineclass.go` that allows re-queuing of machines which refer to an modified providerMachineClass.
