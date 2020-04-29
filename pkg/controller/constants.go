package controller

// TODO: Split and move this constants to provider-specific machine-controllers in future.
const (
	// MachineTypeNotAvailableAzure is a log message depicting certain machine-types are not available in the azure-cloud.
	MachineTypeNotAvailableAzure = "The requested size for resource '.*' is currently not available in location '.*' zones '.*' for subscription '.*'"

	// MachineTypeNotAvailableAWS is a log message depicting certain machine-types are not available in the azure-cloud.
	MachineTypeNotAvailableAWS = "Unsupported: Your requested instance type (.*) is not supported in your requested Availability Zone (.*). Please retry your request by not specifying an Availability Zone or choosing .*"

	// MachineTypeNotAvailableAnnotation annotation is put on machine and node obejcts when cloud-provider is out of certain machine-types.
	MachineTypeNotAvailableAnnotation = "machine.sapcloud.io/machine-type-not-available"
)

// Original error messages:
// AWS:
// Cloud provider message - Unsupported: Your requested instance type (m5.4xlarge) is not supported in your requested Availability Zone (us-east-1e). Please retry your request by not specifying an Availability Zone or choosing us-east-1b, us-east-1d, us-east-1a, us-east-1f, us-east-1c.

// Azure:
// The requested size for resource '/subscriptions/123c2-4-1234/resourceGroups/shoot--12-12/providers/Microsoft.Compute/virtualMac2321321312/1234-nz7vw' is currently not available in location 'westeurope' zones '1' for subscription '2c3d1231-s21321321-8c04-2711234'.
