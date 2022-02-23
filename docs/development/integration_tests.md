# Integration tests

## Usage

## General setup & configurations

Integration tests for `machine-controller-manager-provider-{provider-name}` can be executed manually by following below steps.

1. Clone the repository `machine-controller-manager-provider-{provider-name}` on the local system.
1. Navigate to `machine-controller-manager-provider-{provider-name}` directory and create a `dev` sub-directory in it.
1. Copy the kubeconfig of Control Cluster  from into `dev/control-kubeconfig.yaml`. 
1. (optional) Copy the kubeconfig of Target Cluster  into `dev/target-kubeconfig.yaml` and update the `Makefile` variable `TARGET_KUBECONFIG` to point to `dev/target-kubeconfig.yaml`.
1. If the tags on instances & associated resources on the provider are of `String` type (for example, GCP tags on its instances are of type `String` and not key-value pair) then add `TAGS_ARE_STRINGS := true` in the `Makefile` and export it.
1. Atleast, one of the two controllers' container images must be set in the `Makefile` variables `MCM_IMAGE_TAG` and `MC_IMAGE_TAG` for the controllers to run in the Control Cluster . These images will be used along with `kubernetes/deployment.yaml` to deploy/update controllers in the Control Cluster . If the intention is to run the controllers locally then unset the variables `MCM_IMAGE_TAG` and `MC_IMAGE_TAG` and set variable `MACHINE_CONTROLLER_MANAGER_DEPLOYMENT_NAME := machine-controller-manager` in the `Makefile`.
7. In order to apply the CRDs when the Control Cluster is a Gardener Shoot or if none of the controller images are specified, `machine-controller-manager` repository will be cloned automatically. Incase, this repository already exists in local system, then create a softlink as below which helps to test changes in `machine-controller-manager` quickly.
    ```bash
    ln -sf <path-for-machine-controller-manager-repo> dev/mcm
    ```
## Scenario based additional configurations
### Gardener Shoot as the Control Cluster 

If the Control Cluster  is a Gardener Shoot cluster then,

1. Deploy a `Secret` named `test-mc-secret` (that contains the provider secret and cloud-config) in the `default` namespace of the Control Cluster. Refer [these](https://github.com/gardener/machine-controller-manager/tree/master/kubernetes/machine_classes) `MachineClass` templates for the same.
1. Create a `dev/machineclassv1.yaml` file in the cloned repository. The name of the `MachineClass` itself should be `test-mc-v1`. The value of `providerSpec.secretRef.name` should be `test-mc-secret`. 
1. (Optional) Create an additional `dev/machineclassv2.yaml` file similar to above but with a bigger machine type and update the `Makefile` variable `MACHINECLASS_V2` to point to `dev/machineclassv2.yaml`.

### Gardener Seed as the Control Cluster 

If the Control Cluster  is a Gardener SEED cluster then, the suite ideally employs the already existing `MachineClass` and Secrets. However,

1. (Optional) User can employ a custom `MachineClass` for the tests using below steps:
    1. Deploy a `Secret` named `test-mc-secret` (that contains the provider secret and cloud-config) in the shoot namespace of the Control Cluster. That is, the value of `metadata.namespace` should be `technicalID` of the Shoot and it will be of the pattern `shoot--<project>--<shoot-name>`. Refer [these](https://github.com/gardener/machine-controller-manager/tree/master/kubernetes/machine_classes) `MachineClass` templates for the same. 
    1. Create a `dev/machineclassv1.yaml` file.
        1. `providerSpec.secretRef.name` should refer the secret created in the previous step.
        1. `metadata.namespace` and `providerSpec.secretRef.namespace` should be `technicalID` (`shoot--<project>--<shoot-name>`) of the shoot.
        1.  The name of the `MachineClass` itself should be `test-mc-v1`.

## Running the tests

1. There is a rule `test-integration` in the `Makefile`, which can be used to start the integration test:
    ```bash
    $ make test-integration 
    Starting integration tests...
    Running Suite: Controller Suite
    ===============================
    ```
1. The controllers log files (`mcm_process.log` and `mc_process.log`) are stored in `.ci/controllers-test/logs` repo and can be used later.
## Adding Integration Tests for new providers

For a new provider, Running Integration tests works with no changes. But for the orphan resource test cases to work correctly, the provider-specific API calls and the Resource Tracker Interface (RTI) should be implemented. Please check [`machine-controller-manager-provider-aws`](https://github.com/gardener/machine-controller-manager-provider-aws/blob/master/test/integration/provider/) for reference.

## Extending integration tests

- Update [ControllerTests](../../pkg/test/integration/common/framework.go#L481) to be extend the testcases for all providers. Common testcases for machine|machineDeployment creation|deletion|scaling are packaged into [ControllerTests](../../pkg/test/integration/common/framework.go#L481).
- To extend the provider specfic test cases, the changes should be done in the `machine-controller-manager-provider-{provider-name}` repository. For example, to extended the testcases for `machine-controller-manager-provider-aws`, make changes to `test/integration/controller/controller_test.go` inside the `machine-controller-manager-provider-aws` repository. `commons` contains the `Cluster` and `Clientset` objects that makes it easy to extend the tests.