# Handling VM properties which DON'T trigger rolling update(specifically tags)

## Current Situation with Tags

We pass the `labels` which user provides in [shootYaml](https://github.com/gardener/gardener/blob/fb29d38e6615ed17d409a8271a285254d9dd00ad/example/90-shoot.yaml#L61-L62) as `tags` to the VM.
For doing that we update the machineClass. This update is done by gardener-extension-provider-XYZ.
Currently 
- in aws provider directly passes the labels to the [tag section](https://github.com/gardener/gardener-extension-provider-aws/blob/0a740eeca301320275d77d1c48d3c32d4ebcd7dd/pkg/controller/worker/machines.go#L158-L164) of machineClass
- in [azure](https://github.com/gardener/gardener-extension-provider-azure/blob/b6424f0122e174863e783555aa0ad68700edd87b/pkg/controller/worker/machines.go#L371-L373) sanitize them, and then pass. 
- in [GCP](https://github.com/gardener/gardener-extension-provider-gcp/blob/eb851f716e45336b486f3aaf46268859de2adecb/pkg/controller/worker/machines.go#L312-L315) sanitize them, and then pass.

Looking at an example of what machineClass tag section(from aws) looks like:
```
tags:
    kubernetes.io/arch: amd64
    networking.gardener.cloud/node-local-dns-enabled: "true"
    node.kubernetes.io/role: node
    worker.garden.sapcloud.io/group: worker-1
    worker.gardener.cloud/cri-name: containerd
    worker.gardener.cloud/pool: worker-1
    worker.gardener.cloud/system-components: "true"
    
    kubernetes.io/cluster/shoot--i544024--rolling-test: "1"
    kubernetes.io/role/node: "1"     
                              
    testlabel: "true"                                          
```

section1 -> [g/g](https://github.com/gardener/gardener/blob/c11c86ae07d8ea784f5c41362cd41800f06bb3ed/pkg/operation/botanist/component/extensions/worker/worker.go#L171-L197)</br>
section2 -> [extension-provider](https://github.com/gardener/gardener-extension-provider-aws/blob/0a740eeca301320275d77d1c48d3c32d4ebcd7dd/pkg/controller/worker/machines.go#L160-L161)</br>
section3 -> by user

Out of these , MCM needs to put following tags(calling it `Must Tags`) on the VM for its orphan collection and GetVMStatus logic to work, can be seen in section2 above:
```
Name: <machineObj name> (only for AWS case as VM name differs from machineObj name)
kubernetes.io/cluster/<cluster-full-name>: 1
kuberenetes.io/role/<node/integration-test>: 1
```


These tags transported to :
AWS     -> [VM , Disks, Nics](https://github.com/gardener/machine-controller-manager-provider-aws/blob/0e4162b4bb50d555c831a294af89b5d1c62f8749/pkg/aws/core.go#L115-L128)</br>
Azure   -> [VM,Disks(can't find in code but on portal),Nics](https://github.com/gardener/machine-controller-manager-provider-azure/blob/6488dfe8ed7efb46308aca22055421f3a4026c79/pkg/azure/utils.go#L116)</br>
GCP     -> [VM](https://github.com/gardener/machine-controller-manager-provider-gcp/blob/a82afc613e26e8088244b431b1d89fa9a65e99f3/pkg/gcp/machine_controller_util.go#L70),[Disks(only 1 tag with cluster name is added)](https://github.com/gardener/gardener-extension-provider-gcp/blob/12c157a2a2af040fd9d5cdf5260548f30b2c518c/pkg/controller/worker/machines.go#L292-L294)</br>

In GCP key-value pairs are called `labels` while single key is called `network tag`. We add `Must Tags` to `network tags` in GCP as of now.
Also as you notice the `Must Tags` are not added only Disks and Nics currently. Thats because they have autoDelete enabled , so if VM is deleted then nics and disks are too.

### Problems with the current logic
- <span style="color:red">We don't update the tags once machine is created</span>.
This is because we refer to the tags in machineClass only during machineCreation.
- We don't allow user to add tags just to VM, disks and nics seperately, they are added almost every time on all. This could exhaust the max limit very quickly disallowing user from adding relevant tags.
- We pass labels(which are supposed to just be on nodes) unnecessarily as tags 


## Proposed soln

Currently Worker Spec in shoot Yaml has ProviderConfig as [runtime.rawExtension](https://github.com/gardener/gardener/blob/9a02394eccf2c50e3f1ae23188c219fead5a1402/pkg/apis/core/v1beta1/types_shoot.go#L1298)
Here we could provide provider specific configurations , but its also used to compute [Hash]() which is suffixed to the machineClass name. So if we add `tags` section here then update in them could lead to update in machineClass name and finally rolling update.
Internally ProviderConfig holds values for the struct [WorkerConfig]()(AWS example). 

We propose the following solution: 

### On Gardener level 

In `WorkerConfig` struct, the fields which could be updated without replacement of the VM, would be marked as `updatable:true` (others can be `updatable:false`)
While calculating [WorkerPoolHash](https://github.com/gardener/gardener/blob/d9376c117efb8f31334131cf1d995d99fc2f51d4/extensions/pkg/controller/worker/machines.go#L146-L148)
We will pass the fields with `updatable:false` tags, as additionalData in `WorkerPoolHash()`.
This way an update in these fields would not change the machineClass name and they would also be updated in the machineClass.

### On MCM level

- Every machineClass update pushes the machines referring that machineClass to queue for reconciliation.
- Machineclass has `last-updated-machineClass` annotation. The value of this anno is the machineClass that has been applied for the machines successfully, VM/Nic/Disk parameters are as per this.
- We introduce a new driver call named `UpdateMachine`. The mcm-provider-XYZ has to implement it. If its not implemented, updatable/non-rolling-update parameters of the machine won't be updated.
- We call `UpdateMachine` in `reconcileClusterMachine` only when the hash of current machineClass doesn't match the `last-updated-machineClass`
- Error returned by `UpdateMachine` should be updated in the status of the machine object, and retry period should be according to the type of error(recoverable or long-recoverable)
- If no error returned , `last-updated-machineClass` anno is updated with the current machineClass value.

#### Implemenation of UpdateMachine hints

Inside `UpdateMachine`:
- the current machineClass providerSpec has to be unmarshalled inside mcm-provider-xyz
- the `last-applied-machineClass` providerSpec also has to be unmarshalled
- the logic should compare the updatable/non-rolling-update fields(they should be marked with `updatable:"true"` struct tag)
- if there is a difference , then call the provider API for UpdateVM/nic/disk
- if the fields can't be updated with single provider API call, their updates should happen serially,with minimum provider calls, and error should be returned.
  - error returning could be as soon as an error occurs
  - or it could be combined error from all api calls
- the error(s) should be classified as `recoverable error` or `long-recoverable error`


#### Allowing seperate set of tags for VM, Nics , Disks

This would require update of machineClass. Currently there is just `tags` field available in most providerSpec (`labels` in gcp also present).
This field is just to update tags for VM, and the same is applied on disks and nics.
To allow seperate tags , `tags` field should be deprecated and new fields like `vmTags`, `nicTags`, `diskTags` needs to be introduced.
Necessary changes would be required in Gardener extensions as well.


