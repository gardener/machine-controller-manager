# Hot-Update VirtualMachine tags without triggering a rolling-update

## Motivation

* [MCM Issue#750](https://github.com/gardener/machine-controller-manager/issues/750) There is a requirement to provide a way for consumers to add tags which can be hot-updated onto VMs. This requirement can be generalized to also offer a convenient way to specify tags which can be applied to VMs, NICs, Devices etc.

* [MCM Issue#635](https://github.com/gardener/machine-controller-manager/issues/635) which in turn points to [MCM-Provider-AWS Issue#36](https://github.com/gardener/machine-controller-manager-provider-aws/issues/36#issuecomment-677530395) - The issue hints at other fields like enable/disable [source/destination checks for NAT instances](https://docs.aws.amazon.com/vpc/latest/userguide/VPC_NAT_Instance.html#EIP_Disable_SrcDestCheck) which needs to be hot-updated on network interfaces.

* In GCP provider - `instance.ServiceAccounts` can be updated without the need to roll-over the instance. [See](https://cloud.google.com/compute/docs/access/service-accounts)



## Boundary Condition

All tags that are added via means other than MachineClass.ProviderSpec should be preserved as-is. Only updates done to tags in `MachineClass.ProviderSpec` should be applied to the infra resources (VM/NIC/Disk).

## What is available today?

WorkerPool configuration inside [shootYaml](https://github.com/gardener/gardener/blob/fb29d38e6615ed17d409a8271a285254d9dd00ad/example/90-shoot.yaml#L61-L62) provides a way to set labels. As per the [definition](https://gardener.cloud/docs/gardener/api-reference/core/#core.gardener.cloud/v1beta1.Worker) these labels will be applied on `Node` resources. Currently these labels are also passed to the VMs as tags. There is no distinction made between `Node` labels and `VM` tags.

`MachineClass` has a field which holds [provider specific configuration](https://github.com/gardener/machine-controller-manager/blob/master/pkg/apis/machine/v1alpha1/machineclass_types.go#L54) and one such configuration is `tags`. Gardener provider extensions updates the tags in `MachineClass`.

* AWS provider extension directly passes the labels to the [tag section](https://github.com/gardener/gardener-extension-provider-aws/blob/0a740eeca301320275d77d1c48d3c32d4ebcd7dd/pkg/controller/worker/machines.go#L158-L164) of machineClass.
* Azure provider extension [sanitizes](https://github.com/gardener/gardener-extension-provider-azure/blob/b6424f0122e174863e783555aa0ad68700edd87b/pkg/controller/worker/machines.go#L371-L373) the woker pool labels and adds them as [tags in MachineClass](https://github.com/gardener/gardener-extension-provider-azure/blob/b6424f0122e174863e783555aa0ad68700edd87b/pkg/controller/worker/machines.go#L187).
* GCP provider extension [sanitizes](https://github.com/gardener/gardener-extension-provider-gcp/blob/eb851f716e45336b486f3aaf46268859de2adecb/pkg/controller/worker/machines.go#L312-L315) them, and then sets them as [labels in the MachineClass](https://github.com/gardener/gardener-extension-provider-gcp/blob/eb851f716e45336b486f3aaf46268859de2adecb/pkg/controller/worker/machines.go#L169). In GCP tags only have keys and are currently [hard coded](https://github.com/gardener/gardener-extension-provider-gcp/blob/eb851f716e45336b486f3aaf46268859de2adecb/pkg/controller/worker/machines.go#L204-L207).

Let us look at an example of `MachineClass.ProviderSpec` in AWS:

```yaml
providerSpec:
  ami: ami-02fe00c0afb75bbd3
  tags:
    #[section-1] pool lables added by gardener extension
    #########################################################
    kubernetes.io/arch: amd64
    networking.gardener.cloud/node-local-dns-enabled: "true"
    node.kubernetes.io/role: node
    worker.garden.sapcloud.io/group: worker-ser234
    worker.gardener.cloud/cri-name: containerd
    worker.gardener.cloud/pool: worker-ser234
    worker.gardener.cloud/system-components: "true"

    #[section-2] Tags defined in the gardener-extension-provider-aws
    ###########################################################
    kubernetes.io/cluster/cluster-full-name: "1"
    kubernetes.io/role/node: "1"

    #[section-3]
    ###########################################################
    user-defined-key1: user-defined-val1
    user-defined-key2: user-defined-val2
```

>  Refer [src](https://github.com/gardener/gardener/blob/c11c86ae07d8ea784f5c41362cd41800f06bb3ed/pkg/operation/botanist/component/extensions/worker/worker.go#L171-L197) for tags defined in `section-1`.
>  Refer [src](https://github.com/gardener/gardener-extension-provider-aws/blob/0a740eeca301320275d77d1c48d3c32d4ebcd7dd/pkg/controller/worker/machines.go#L158-L164) for tags defined in `section-2`.
>  Tags in `section-3` are defined by the user.

Out of the above three tag categories, MCM depends `section-2` tags (`mandatory-tags`) for its `orphan collection` and Driver's `DeleteMachine`and `GetMachineStatus` to work. 

`ProviderSpec.Tags` are transported to the provider specific resources as follows:

| Provider | Resources Tags are set on                                                      | Code Reference                                                                                                                                                                                                                                                           | Comment                                                                        |
| -------- | ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------ |
| AWS      | Instance(VM), Volume, Network-Interface                                        | [aws-VM-Vol-NIC](https://github.com/gardener/machine-controller-manager-provider-aws/blob/v0.17.0/pkg/aws/core.go#L116-L129)                                                                                                                                             | No distinction is made between tags set on VM, NIC or Volume                   |
| Azure    | Instance(VM), Network-Interface                                                | [azure-VM-parameters](https://github.com/gardener/machine-controller-manager-provider-azure/blob/v0.10.0/pkg/azure/utils.go#L234) & [azureNIC-Parameters](https://github.com/gardener/machine-controller-manager-provider-azure/blob/v0.10.0/pkg/azure/utils.go#L116)    |                                                                                |
| GCP      | Instance(VM), 1 tag: `name` (denoting the name of the worker) is added to Disk | [gcp-VM](https://github.com/gardener/machine-controller-manager-provider-gcp/blob/v0.14.0/pkg/gcp/machine_controller_util.go#L78-L80) & [gcp-Disk](https://github.com/gardener/gardener-extension-provider-gcp/blob/v1.28.1/pkg/controller/worker/machines.go#L291-L293) | In GCP key-value pairs are called `labels` while `network tags` have only keys |
| AliCloud | Instance(VM)                                                                   | [aliCloud-VM](https://github.com/gardener/machine-controller-manager-provider-alicloud/blob/master/pkg/spi/spi.go#L125-L129)                                                                                                                                             |                                                                                |

## What are the problems with the current approach?

There are a few shortcomings in the way tags/labels are handled:

* Tags can only be set at the time a machine is created.
* There is no distinction made amongst tags/labels that are added to VM's, disks or network interfaces. As stated above for AWS same set of tags are added to all. There is a limit defined on the number of tags/labels that can be associated to the devices (disks, VMs, NICs etc). Example: In AWS a max of [50 user created tags are allowed](https://docs.aws.amazon.com/general/latest/gr/aws_tagging.html). Similar restrictions are applied on different resources across providers. Therefore adding all tags to all devices even if the subset of tags are not meant for that resource exhausts the total allowed tags/labels for that resource.
* The only placeholder in shoot yaml as mentioned above is meant to only hold labels that should be applied on primarily on the [Node](https://github.com/gardener/gardener/blob/v1.66.1/pkg/apis/core/v1beta1/types_shoot.go#L1315-L1317) objects. So while you could use the node labels for [extended resources](https://github.com/gardener/machine-controller-manager/issues/727), using it also for tags is not clean.
* There is no provision in the shoot YAML today to add tags only to a subset of resources.

### MachineClass Update and its impact

When [Worker.ProviderConfig](https://github.com/gardener/gardener/blob/v1.66.1/pkg/apis/core/types_shoot.go#L1042-L1043) is changed then a [worker-hash](https://github.com/gardener/gardener/blob/v1.66.1/extensions/pkg/controller/worker/machines.go#L146-L148) is computed which includes the raw `ProviderConfig`. This hash value is then used as a suffix when constructing the name for a `MachineClass`. See [aws-extension-provider](https://github.com/gardener/gardener-extension-provider-aws/blob/master/pkg/controller/worker/machines.go#L190) as an example. A change in the name of the `MachineClass` will then in-turn trigger a rolling update of machines. Since `tags` are provider specific and therefore will be part of `ProviderConfig`, any update to them will result in a rolling-update of machines.

## Proposal

### Shoot YAML changes

Provider specific configuration is set via [providerConfig](https://github.com/gardener/gardener/blob/master/example/90-shoot.yaml#L57-L58) section for each worker pool.

Example worker provider config (current):

```yaml
providerConfig:
   apiVersion: aws.provider.extensions.gardener.cloud/v1alpha1
   kind: WorkerConfig
   volume:
     iops: 10000
   dataVolumes:
   - name: kubelet-dir
     snapshotID: snap-13234
   iamInstanceProfile: # (specify either ARN or name)
     name: my-profile
     arn: my-instance-profile-arn
```

It is proposed that an additional field be added for `tags` under `providerConfig`. Proposed changed YAML:

```yaml
providerConfig:
   apiVersion: aws.provider.extensions.gardener.cloud/v1alpha1
   kind: WorkerConfig
   volume:
     iops: 10000
   dataVolumes:
   - name: kubelet-dir
     snapshotID: snap-13234
   iamInstanceProfile: # (specify either ARN or name)
     name: my-profile
     arn: my-instance-profile-arn
   tags:
     vm:
       key1: val1
       key2: val2
       ..
     # for GCP network tags are just keys (there is no value associated to them). 
     # What is shown below will work for AWS provider.
     network:
       key3: val3
       key4: val4
```

Under `tags` clear distinction is made between tags for VMs, Disks, network interface etc. Each provider has a different allowed-set of characters that it accepts as key names, has different limits on the tags that can be set on a resource (disk, NIC, VM etc.) and also has a different format (GCP network tags are only keys).

> TODO: 
> 
> * Check if worker.labels are getting added as tags on infra resources. We should continue to support it and double check that these should only be added to VMs and not to other resources. 
> 
> * Should we support users adding VM tags as node labels?

### Provider specific WorkerConfig API changes

> Taking `AWS` provider extension as an example to show the changes.

[WorkerConfig](https://github.com/gardener/gardener-extension-provider-aws/blob/master/pkg/apis/aws/types_worker.go#L27-L38) will now have the following changes:

1. A new field for tags will be introduced.
2. Additional metadata for struct fields will now be added via `struct tags`.

```go
type WorkerConfig struct {
    metav1.TypeMeta
    Volume *Volume
    // .. all fields are not mentioned here.
    // Tags are a collection of tags to be set on provider resources (e.g. VMs, Disks, Network Interfaces etc.)
    Tags *Tags `hotupdatable:true`
}

// Tags is a placeholder for all tags that can be set/updated on VMs, Disks and Network Interfaces.
type Tags struct {
    // VM tags set on the VM instances.
    VM map[string]string
    // Network tags set on the network interfaces.
    Network map[string]string
    // Disk tags set on the volumes/disks.
    Disk map[string]string
}
```

There is a need to distinguish fields within `ProviderSpec` (which is then mapped to the above `WorkerConfig`) which can be updated without the need to change the hash suffix for `MachineClass` and thus trigger a rolling update on machines.

To achieve that we propose to use **struct tag** `hotupdatable` whose value indicates if the field can be updated without the need to do a rolling update. To ensure backward compatibility, all fields which do not have this tag or have `hotupdatable` set to `false` will be considered as immutable and will require a rolling update to take affect.

### Gardener provider extension changes

> Taking AWS provider extension as an example. Following changes should be made to all gardener provider extensions

AWS Gardener Extension [generates machine config](https://github.com/gardener/gardener-extension-provider-aws/blob/v1.42.1/pkg/controller/worker/machines.go#L104-L107) using worker pool configuration. As part of that it also computes the `workerPoolHash` which is then used to create the [name of the MachineClass](https://github.com/gardener/gardener-extension-provider-aws/blob/master/pkg/controller/worker/machines.go#L193).

Currently `WorkerPoolHash` function uses the [entire providerConfig](https://github.com/gardener/gardener-extension-provider-aws/blob/47d6bb34a538f3dfeedcf99361696de72d1eeae2/vendor/github.com/gardener/gardener/extensions/pkg/controller/worker/machines.go#L146-L148) to compute the hash. Proposal is to do the following:

1. Remove the [code](https://github.com/gardener/gardener-extension-provider-aws/blob/47d6bb34a538f3dfeedcf99361696de72d1eeae2/vendor/github.com/gardener/gardener/extensions/pkg/controller/worker/machines.go#L146-L148) from function `WorkerPoolHash`.
2. Add another function to compute hash using all immutable fields in the provider config struct and then pass that to `worker.WorkerPoolHash` as `additionalData`.

The above will ensure that tags and any other field in `WorkerConfig` which is marked with `updatable:true` is not considered for hash computation and will therefore not contribute to changing the name of `MachineClass` object thus preventing a rolling update.

`WorkerConfig` and therefore the contained tags will be set as [ProviderSpec](https://github.com/gardener/machine-controller-manager/blob/master/pkg/apis/machine/v1alpha1/machineclass_types.go#L54) in `MachineClass`.

If only fields which have `updatable:true` are changed then it should result in update/patch of `MachineClass` and not creation. 

### Driver interface changes

[Driver](https://github.com/gardener/machine-controller-manager/blob/master/pkg/util/provider/driver/driver.go#L28) interface which is a facade to provider specific API implementations will have one additional method.

```golang
type Driver interface {
    // .. existing methods are not mentioned here for brevity.
    UpdateMachine(context.Context, *UpdateMachineRequest) error
}

// UpdateMachineRequest is the request to update machine tags. 
type UpdateMachineRequest struct {
    ProviderID string
    LastAppliedProviderSpec raw.Extension
    MachineClass *v1alpha1.MachineClass
    Secret *corev1.Secret
}
```

> If any `machine-controller-manager-provider-<providername>` has not implemented `UpdateMachine` then updates of tags on Instances/NICs/Disks will not be done. An error message will be logged instead. 

> 



### Machine Class reconciliation

Current [MachineClass reconciliation](https://github.com/gardener/machine-controller-manager/blob/v0.48.1/pkg/util/provider/machinecontroller/machineclass.go#L140-L194) does not reconcile `MachineClass` resource updates but it only enqueues associated machines. The reason is that it is assumed that anything that is changed in a MachineClass will result in a creation of a new MachineClass with a different name. This will result in a rolling update of all machines using the MachineClass as a template.

However, it is possible that there is data that all machines in a `MachineSet` share which do not require a rolling update (e.g. tags), therefore there is a need to reconcile the MachineClass as well.

#### Reconciliation Changes

In order to ensure that machines get updated eventually with changes to the `hot-updatable` fields defined in the `MachineClass.ProviderConfig` as `raw.Extension`.

We should only fix [MCM Issue#751](https://github.com/gardener/machine-controller-manager/issues/751) in the MachineClass reconciliation and let it enqueue the machines as it does today. We additionally propose the following two things:

1. Introduce a new annotation `last-applied-providerspec` on every machine resource. This will capture the last successfully applied `MachineClass.ProviderSpec` on this instance.

2. Enhance the machine reconciliation to include code to hot-update machine. 

In [machine-reconciliation](https://github.com/gardener/machine-controller-manager/blob/v0.48.1/pkg/util/provider/machinecontroller/machine.go#L114) there are currently two flows `triggerDeletionFlow` and `triggerCreationFlow`. When a machine gets enqueued due to changes in MachineClass then in this method following changes needs to be introduced:


Check if the machine has `last-applied-providerspec` annotation. 

*Case 1.1*

If the annotation is not present then there can be just 2 possibilities:

* It is a fresh/new machine and no backing resources (VM/NIC/Disk) exist yet. The current flow checks if the providerID is empty and `Status.CurrenStatus.Phase` is empty then it enters into the `triggerCreationFlow`.

* It is an existing machine which does not yet have this annotation. In this case call `Driver.UpdateMachine`. If the driver returns no error then add `last-applied-providerspec` annotation with the value of `MachineClass.ProviderSpec` to this machine.

*Case 1.2*

If the annotation is present then compare the last applied provider-spec with the current provider-spec. If there are changes (check their hash values) then call `Driver.UpdateMachine`. If the driver returns no error then add `last-applied-providerspec` annotation with the value of `MachineClass.ProviderSpec` to this machine.

> NOTE: It is assumed that if there are changes to the fields which are not marked as `hotupdatable` then it will result in the change of name for MachineClass resulting in a rolling update of machines. If the name has not changed + machine is enqueued + there is a change in machine-class then it will be change to a hotupdatable fields in the spec.



Trigger update flow can be done after `reconcileMachineHealth` and `syncMachineNodeTemplates` in [machine-reconciliation](https://github.com/gardener/machine-controller-manager/blob/v0.48.1/pkg/util/provider/machinecontroller/machine.go#L164-L175).



There are 2 edge cases that needs attention and special handling:

> Premise: It is identified that there is an update done to one or more hotupdatable fields in the MachineClass.ProviderSpec.

*Edge-Case-1*

In the machine reconciliation, an update-machine-flow is triggered which in-turn calls `Driver.UpdateMachine`. Consider the case where the hot update needs to be done to all VM, NIC and Disk resources. The driver returns an error which indicates a `partial-failure`. As we have mentioned above only when `Driver.UpdateMachine` returns no error will `last-applied-providerspec` be updated. In case of partial failure the annotation will not be updated. This event will be re-queued for a re-attempt. However consider a case where before the item is re-queued, another update is done to MachineClass reverting back the changes to the original spec.

| At T1                                                           | At T2 (T2 > T1)                                                                                                                               | At T3 (T3> T2)                                                                                                               |
| --------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| last-applied-providerspec=S1<br/>MachineClass.ProviderSpec = S1 | last-applied-providerspec=S1<br/>MachineClass.ProviderSpec = S2<br/> Another update to MachineClass.ProviderConfig = S3 is enqueue (S3 == S1) | last-applied-providerspec=S1<br/>Driver.UpdateMachine for S1-S2 update - returns partial failure<br/>Machine-Key is requeued |

At T4 (T4> T3) when a machine is reconciled then it checks that `last-applied-providerspec` is S1 and current MachineClass.ProviderSpec = S3 and since S3 is same as S1, no update is done. At T2 Driver.UpdateMachine was called to update the machine with `S2` but it partially failed. So now you will have resources which are partially updated with S2 and no further updates will be attempted.



*Edge-Case-2*

The above situation can also happen when `Driver.UpdateMachine` is in the process of updating resources. It has hot-updated lets say 1 resource. But now MCM crashes. By the time it comes up another update to MachineClass.ProviderSpec is done essentially reverting back the previous change (same case as above). In this case reconciliation loop never got a chance to get any response from the driver.



To handle the above edge cases there are 2 options:



*Option #1* 

Introduce a new annotation `inflight-providerspec-hash` . The value of this annotation will be the hash value of the `MachineClass.ProviderSpec` that is in the process of getting applied on this machine. The machine will be updated with this annotation just before calling `Driver.UpdateMachine` (in the trigger-update-machine-flow). If the driver returns no error then (in a single update):

1. `last-applied-providerspec` will be updated

2. `inflight-providerspec-hash` annotation will be removed.



*Option #2* - Preferred 

Leverage `Machine.Status.LastOperation` with `Type` set to `MachineOperationUpdate` and `State` set to `MachineStateProcessing` This status will be updated just before calling `Driver.UpdateMachine`.

Semantically `LastOperation` captures the details of the operation post-operation and not pre-operation. So this solution would be a divergence from the norm.
