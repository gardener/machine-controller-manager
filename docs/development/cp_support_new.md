# Adding support for a new provider

Steps to be followed while implementing a new (hyperscale) provider are mentioned below. This is the easiest way to add a new provider support using a blueprint code.

However, you may also develop your machine controller from scratch which would provide you more flexibility. However make sure that your custom machine controller adhere's to the `Machine.Status` struct defined in the [MachineAPIs](/pkg/apis/machine/types.go) to make sure the MCM is able to act with higher level controllers like MachineSet and MachineDeployment controller. The key is the `Machine.Status.CurrentStatus.Phase` key that indicates the status of the machine object.

Our strong recommendation would be to follow the steps below as this provides most flexibility required to support machine management for adding new providers. And if you feel to extend the functionality feel free to update our [machine controller libraries](/pkg/util/provider).

## Setting up your repository

1. Create a new empty repository named `machine-controller-manager-provider-{provider-name}` on github username/project. Do not initialize this repository with a README.
1. Copy the remote repository `URL` (HTTPS/SSH) to this repository which is displayed once you create this repository.
1. Now on your local system, create directories as required. {your-github-username} given below could also be {github-project} depending on where you have created the new repository.
    ```bash
    mkdir -p $GOPATH/src/github.com/{your-github-username}
    ```
1. Navigate to this created directory.
    ```bash
    cd $GOPATH/src/github.com/{your-github-username}
    ```
1. Clone [this repository](https://github.com/gardener/machine-controller-manager-provider-sampleprovider) on your local machine.
    ```bash
    git clone git@github.com:gardener/machine-controller-manager-provider-sampleprovider.git
    ```
1. Rename the directory from `machine-controller-manager-provider-sampleprovider` to `machine-controller-manager-provider-{provider-name}`.
    ```bash
    mv machine-controller-manager-provider-sampleprovider machine-controller-manager-provider-{provider-name}
    ```
1. Navigate into the newly created directory.
    ```bash
    cd machine-controller-manager-provider-{provider-name}
    ```
1. Update the remote `origin` URL to the newly created repository's URL you had copied above.
    ```bash
    git remote set-url origin git@github.com:{your-github-username}/machine-controller-manager-provider-{provider-name}.git
    ```
1. Rename github project from `gardener` to `{github-org/your-github-username}` wherever you have cloned the repository above. Also edit all occurrences of the word `sampleprovider` to `{provider-name}` in the code. Use the hack script given below to do the same.
    ```bash
    make rename-project PROJECT_NAME={github-org/your-github-username} PROVIDER_NAME={provider-name}
    eg:
        make rename-project PROJECT_NAME=gardener PROVIDER_NAME=AmazonWebServices (or)
        make rename-project PROJECT_NAME=githubusername PROVIDER_NAME=AWS
    ```
1. Now commit your changes and push it upstream.
    ```bash
    git add -A
    git commit -m "Renamed SampleProvide to {provider-name}"
    git push origin master
    ```

## Code changes required

The contract between he Machine Controller Manager (MCM) and the Machine Controller (MC) AKA driver has been [documented here](machine_error_codes.md) and the [machine error codes can be found here](/pkg/util/provider/machinecodes/codes/codes.go). You may refer to them for any queries.

:warning:
- Keep in mind that, **there should to be a unique way to map between machine objects and VMs**. This can be done by mapping machine object names with VM-Name/ tags/ other metadata.
- Optionally there should also be a unique way to map a VM to it's machine class object. This can be done by tagging VM objects with tags/resource-groups associated with the machine class.

#### Steps to integrate

1. Update the `pkg/provider/apis/provider_spec.go` specification file to reflect the structure of the `ProviderSpec` blob. It typically contains the machine template details in the `MachineClass` object. Follow the sample spec provided already in the file. A sample provider specification can be found [here](https://github.com/gardener/machine-controller-manager-provider-aws/blob/master/pkg/aws/apis/aws_provider_spec.go).
1. Fill in the methods described at `pkg/provider/core.go` to manage VMs on your cloud provider. Comments are provided above each method to help you fill them up with desired `REQUEST` and `RESPONSE` parameters.
    - A sample provider implementation for these methods can be found [here](https://github.com/gardener/machine-controller-manager-provider-aws/blob/master/pkg/aws/core.go).
    - Fill in the required methods `CreateMachine()`, and `DeleteMachine()` methods.
    - Optionally fill in methods like `GetMachineStatus()`, `ListMachines()`, and `GetVolumeIDs()`. You may choose to fill these, once the working of the required methods seem to be working.
        - `GetVolumeIDs()` expects VolumeIDs to be decoded from the volumeSpec based on the cloud provider.
    - There is also an OPTIONAL method `GenerateMachineClassForMigration()` that helps in migration of `{ProviderSpecific}MachineClass` to `MachineClass` CR (custom resource). This only makes sense if you have an existing implementation (in-tree) acting on different CRD types and you would like to migrate this. If not you MUST return an error (machine error UNIMPLEMENTED) to avoid processing this step.
1. Perform validation of APIs that you have described and make it a part of your methods as required at each requests.
1. Write unit tests to make it work with your implementation by running `make test`.
    ```bash
    make test
    ```
1. Re-generate the vendors, to update any new vendors imported.
    ```bash
    make revendor
    ```
1. Update the sample YAML files on `kubernetes/` directory to provide sample files through which the working of the machine controller can be tested.
1. Update `README.md` to reflect any additional changes

## Testing your code changes

Make sure `$TARGET_KUBECONFIG` points to the cluster where you wish to manage machines. `$CONTROL_NAMESPACE` represents the namespaces where MCM is looking for machine CR objects, and `$CONTROL_KUBECONFIG` points to the cluster which holds these machine CRs.

1. On the first terminal running at `$GOPATH/src/github.com/{github-org/your-github-username}/machine-controller-manager-provider-{provider-name}`,
    - Run the machine controller (driver) using the command below.
        ```bash
        make start
        ```
1. On the second terminal pointing to `$GOPATH/src/github.com/gardener`,
    - Clone the [latest MCM code](https://github.com/gardener/machine-controller-manager)
        ```bash
        git clone git@github.com:gardener/machine-controller-manager.git
        ```
    - Navigate to the newly created directory.
        ```bash
        cd machine-controller-manager
        ```
    - Deploy the required CRDs from the machine-controller-manager repo,
        ```bash
        kubectl apply -f kubernetes/crds.yaml
        ```
    - Run the machine-controller-manager in the `cmi-client` branch
        ```bash
        make start
        ```
1. On the third terminal pointing to `$GOPATH/src/github.com/{github-org/your-github-username}/machine-controller-manager-provider-{provider-name}`
    - Fill in the object files given below and deploy them as described below.
    - Deploy the `machine-class`
        ```bash
        kubectl apply -f kubernetes/machine-class.yaml
        ```
    - Deploy the `kubernetes secret` if required.
        ```bash
        kubectl apply -f kubernetes/secret.yaml
        ```
    - Deploy the `machine` object and make sure it joins the cluster successfully.
        ```bash
        kubectl apply -f kubernetes/machine.yaml
        ```
    - Once machine joins, you can test by deploying a machine-deployment.
    - Deploy the `machine-deployment` object and make sure it joins the cluster successfully.
        ```bash
        kubectl apply -f kubernetes/machine-deployment.yaml
        ```
    - Make sure to delete both the `machine` and `machine-deployment` object after use.
        ```bash
        kubectl delete -f kubernetes/machine.yaml
        kubectl delete -f kubernetes/machine-deployment.yaml
        ```

## Releasing your docker image

1. Make sure you have logged into gcloud/docker using the CLI.
2. To release your docker image, run the following.
```bash
    make release IMAGE_REPOSITORY=<link-to-image-repo>
```
3. A sample kubernetes deploy file can be found at `kubernetes/deployment.yaml`. Update the same (with your desired MCM and MC images) to deploy your MCM pod.
