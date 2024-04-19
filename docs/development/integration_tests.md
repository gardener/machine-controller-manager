# Integration tests

## Usage

## General setup & configurations

Integration tests for `machine-controller-manager-provider-{provider-name}` can be executed manually by following below steps.

1. Clone the repository `machine-controller-manager-provider-{provider-name}` on the local system.
1. Navigate to `machine-controller-manager-provider-{provider-name}` directory and create a `dev` sub-directory in it.
1. If the tags on instances & associated resources on the provider are of `String` type (for example, GCP tags on its instances are of type `String` and not key-value pair) then add `TAGS_ARE_STRINGS := true` in the `Makefile` and export it. For GCP this has already been hard coded in the `Makefile`.

## Running the tests

1. There is a rule `test-integration` in the `Makefile`, which can be used to start the integration test:
    ```bash
    $ make test-integration 
    ```
1. This will ask for additional inputs. Most of them are self explanatory except:
 - In case of non-gardener setup (control cluster is not a gardener seed), the name of the machineclass must be `test-mc-v1` and the value of `providerSpec.secretRef.name` should be `test-mc-secret`.
 - In case of azure, `TARGET_CLUSTER_NAME` must be same as the name of the Azure ResourceGroup for the cluster.
 - If you are deploying the secret manually, a `Secret` named `test-mc-secret` (that contains the provider secret and cloud-config) in the `default` namespace of the Control Cluster should be created.
3. The controllers log files (`mcm_process.log` and `mc_process.log`) are stored in `.ci/controllers-test/logs` repo and can be used later.
## Adding Integration Tests for new providers

For a new provider, Running Integration tests works with no changes. But for the orphan resource test cases to work correctly, the provider-specific API calls and the Resource Tracker Interface (RTI) should be implemented. Please check [`machine-controller-manager-provider-aws`](https://github.com/gardener/machine-controller-manager-provider-aws/blob/master/test/integration/provider/) for reference.

## Extending integration tests

- Update [ControllerTests](../../pkg/test/integration/common/framework.go#L481) to be extend the testcases for all providers. Common testcases for machine|machineDeployment creation|deletion|scaling are packaged into [ControllerTests](../../pkg/test/integration/common/framework.go#L481).
- To extend the provider specfic test cases, the changes should be done in the `machine-controller-manager-provider-{provider-name}` repository. For example, to extended the testcases for `machine-controller-manager-provider-aws`, make changes to `test/integration/controller/controller_test.go` inside the `machine-controller-manager-provider-aws` repository. `commons` contains the `Cluster` and `Clientset` objects that makes it easy to extend the tests.
