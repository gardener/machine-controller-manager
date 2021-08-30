# Running Integration tests

Integration tests for `machine-controller-manager` and `machine-controller-manager-provider-{provider-name}` can be executed manually by following below steps.

- Clone the repository `machine-controller-manager-provider-{provider-name}` on your local system.
- Navigate to `machine-controller-manager-provider-{provider-name}` directory and then create a `dev` sub-directory in it.
- Copy the kubeconfig of the kubernetes cluster from where you wish to manage the machines into `dev/control-kubeconfig.yaml`. 
- (optional) Copy the kubeconfig of the kubernetes cluster where you wish to deploy the machines into `dev/target-kubeconfig.yaml`. If you do this, also update the `Makefile` variable TARGET_KUBECONFIG to point to `dev/target-kubeconfig.yaml`.
- If the tags on instances & associated resources on the provider are of string type (for example, GCP tags on its instances are of type string and not key-value pair) then add `TAGS_ARE_STRINGS := true` in the `Makefile`.
- If tags for controller's container images are known, update the `Makefile` variables MCM_IMAGE_TAG and MC_IMAGE_TAG accordingly. These images will be used along with `kubernetes/deployment.yaml` to deploy/update controllers in the cluster. If none of the images are specified, the controllers will be started in the local system. 
- If the kubernetes cluster referred by `dev/control-kubeconfig.yaml` is a gardener SHOOT cluster, then
    - Deploy a secret named `test-mc-secret` that contains the provider secret and cloud-config into the kubernetes cluster. Refer [these](https://github.com/gardener/machine-controller-manager/tree/master/kubernetes/machine_classes) machineClass templates for the same.
    - Create a `dev/machineclassv1.yaml` file. The value of `providerSpec.secretRef.name` should be `test-mc-secret`. The name of the machineclass itself should be `test-mc-v1`. 
    - (optional) Create an additional `dev/machineclassv2.yaml` file similar to above but with a bigger machine type. If you do this, update the `Makefile` variable MACHINECLASS_V2 to point to `dev/machineclassv2.yaml`. 
- If the cluster referred by `dev/control-kubeconfig.yaml` is a gardener SEED cluster then
  - (optional) If you want to use a machineClass of your own for the test
    - Deploy a secret named `test-mc-secret` that contains the provider secret and cloud-config into the kubernetes cluster. Refer [these](https://github.com/gardener/machine-controller-manager/tree/master/kubernetes/machine_classes) machineClass templates for the same. Make sure that value of `metadata.namespace` is the technicalID i.e `shoot--<project>--<shoot-name>` of the shoot.
    - Create a `dev/machineclassv1.yaml` file.
      -  The value of `providerSpec.secretRef.name` should be the secret created in the previous step. 
      -  The value of `providerSpec.secretRef.namespace` should be technical-ID(`shoot--<project>--<shoot-name>`) of the shoot 
      -  The name of the machineclass itself should be `test-mc-v1`. 
      -  The value of `metadata.namespace`  should be technicalID of the shoot.
  
- If none of the images or only one image is specified, `machine-controller-manager` repository will be cloned automatically. Incase, this repository already exists in local system, then create a softlink as below which helps to test changes in `machine-controller-manager` quickly.
    ```bash
    ln -sf <path-for-machine-controller-manager-repo> dev/mcm
    ```
- There is a rule `test-integration` in the `Makefile`, which can be used to start the integration test:
    ```bash
    $ make test-integration 
    Starting integration tests...
    Running Suite: Controller Suite
    ===============================
    ```
- The controllers log files (mcm_process.log and mc_process.log) are stored as temporary files and can be used later.
    
## Adding integration tests for new providers

For a new provider, [Running Integration tests](#Running-Integration-tests) works with no changes. But for the orphan resource test cases to work correctly, the provider-specific API calls and the rti should be implemented. Please check (`machine-controller-manager-provider-aws`)[https://github.com/gardener/machine-controller-manager-provider-aws] for referrence.

## Extending integration tests

- If the testcases for all providers has to be extended, then [ControllerTests](pkg/test/integration/common/framework.go#L481) should be updated. Common test-cases for machine|machineDeployment creation|deletion|scaling have been packaged into [ControllerTests](pkg/test/integration/common/framework.go#L481). But they can be extended any time.
- If the test cases for a specific provider has to be extended, then the changes should be done in the `machine-controller-manager-provider-{provider-name}` repository. For example, if the tests for `machine-controller-manager-provider-aws` are to be extended then make changes to `test/integration/controller/controller_test.go` inside the `machine-controller-manager-provider-aws` repository. `commons` contains the Cluster and Clientset objects that makes it easy to extend the tests.