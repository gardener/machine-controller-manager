# Running Integration tests

Integration tests for `machine-controller-manager` and `machine-controller-manager-provider-{provider-name}` can be executed manually by following below steps.

1. Clone the repository `machine-controller-manager-provider-{provider-name}` on your local system.
1. Navigate to `machine-controller-manager-provider-{provider-name}` directory and then create a `dev` sub-directory in it.
1. Copy the kubeconfig of the kubernetes cluster from where you wish to manage the machines into `dev/control-kubeconfig.yaml`. 
1. (optional) Copy the kubeconfig of the kubernetes cluster where you wish to deploy the machines into `dev/target-kubeconfig.yaml`. If you do this, also update the `Makefile` variable TARGET_KUBECONFIG to point to `dev/target-kubeconfig.yaml`.
1. If the kubernetes cluster referred by `dev/control-kubeconfig.yaml` is a gardener shoot cluster, then
    - Create a secret that contains the provider secret and cloud-config into the kubernetes cluster.
    - Create a `dev/machineclassv1.yaml` file. The value of `providerSpec.secretRef.name` should be the secret created in the previous step. The name of the machineclass itself should be `test-mc-v1`. 
    - (optional) Create an additional `dev/machineclassv2.yaml` file similar to above but with a bigger machine type. If you do this, update the `Makefile` variable MACHINECLASS_V2 to point to `dev/machineclassv2.yaml`. 
    - If tags for controllers container images are known, update the `Makefile` variables MCM_IMAGE_TAG and MC_IMAGE_TAG accordingly. These will be used along with `kubernetes/deployment.yaml` to deploy controllers into the cluster. If not, the controllers will be started in the local system. `machine-controller-manager` repository will be cloned automatically for this. incase, this repository already exists in local system, then create a softlink as below. It also helps to test changes in `machine-controller-manager` quickly.
    ```bash
    ln -sf <path-for-machine-controller-manager-repo> dev/mcm
    ```
1. If the cluster referred by `dev/control-kubeconfig.yaml` is a gardener seed cluster and the tags for controllers container images are known, update the `Makefile` variables MCM_IMAGE_TAG and MC_IMAGE_TAG accordingly. These will be used to update the existing controllers running the cluster.
1. There is a rule `test-integration` in the `Makefile`, which can be used to start the integration test:
    ```bash
    $ make test-integration 
    Starting integration tests...
    Running Suite: Controller Suite
    ===============================
    ```
1. The controllers log files (mcm_process.log and mc_process.log) are stored as temporary files and can be used later.
    
## Adding integration tests for new providers

For a new provider, [Running Integration tests](#Running-Integration-tests) works with no changes. But for the orphan resource test cases to work correctly, the provider-specific API calls and the rti should be implemented. Please check `machine-controller-manager-provider-aws` for referrence.

## Extending integration tests

- If the testcases for all providers has to be extended, then [ControllerTests](pkg/test/integration/common/framework.go#L481) should be updated. Common test-cases for machine|machineDeployment creation|deletion|scaling have been packaged into [ControllerTests](pkg/test/integration/common/framework.go#L481). But they can be extended any time.
- If the test cases for a specific provider has to be extended, then the changes should be done in the `machine-controller-manager-provider-{provider-name}` repository. For example, if the tests for `machine-controller-manager-provider-aws` are to be extended then make changes to `test/integration/controller/controller_test.go` inside the `machine-controller-manager-provider-aws` repository. `commons` contains the Cluster and Clientset objects that makes it easy to extend the tests.
