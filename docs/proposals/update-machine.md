# Update Machine Instance

## Background

Today the [driver.Driver](https://github.com/gardener/machine-controller-manager/blob/rel-v0.49/pkg/util/provider/driver/driver.go#L28) facade represents the boundary between the the `machine-controller` and its various provider specific implementations.

We have abstract operations for creation/deletion and listing of machines (actually compute instances) but no abstract operation to represent the update of an instance after creation.

We have found this to be necessary for several use cases. Today in the MCM AWS Provider, we already misuse `driver.GetMachineStatus`  which is supposed to be a read-only operation obtaining the status of an instance. 

Each AWS EC2 instance performs source/destination checks by default.
For [EC2 NAT](https://docs.aws.amazon.com/vpc/latest/userguide/VPC_NAT_Instance.html#EIP_Disable_SrcDestCheck) instances
these should be disabled. This is done by issueing
a [ModifyInstanceAttribute](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ModifyInstanceAttribute.html) request with the `SourceDestCheck` set to `false`. The MCM AWS Provider, decodes the  [AWSProviderSpec](https://github.com/gardener/machine-controller-manager-provider-aws/blob/39318bb2b5b4a573fdc77eaf400839d12c4abf59/pkg/aws/apis/aws_provider_spec.go#L63), reads `providerSpec.SrcAndDstChecksEnabled` and correspondingly issues the call to modify the already launched instance.  However, do not this is a one time action after launching the instance. Such a property is not meant to be hot modified again and again.

Similarly, there is a [pending PR](https://github.com/gardener/machine-controller-manager-provider-aws/pull/128) to add the `Ipv6AddessCount` and `Ipv6PrefixCount` to enable the assignment of an ipv6 address and an ipv6 prefix to instances. This requires constructing and issueing an [AssignIpv6Addresses](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_AssignIpv6Addresses.html) request after the instance is started.

We have other uses-cases such as  [MCM Issue#750](https://github.com/gardener/machine-controller-manager/issues/750) where there is a requirement to provide a way for consumers to add tags which can be hot-updated onto instances. This requirement can be generalized to also offer a convenient way to specify tags which can be applied to VMs, NICs, Devices etc.

## Objectives

We will split the fulfilment of this overall need into 2 stages of implementation.

1. **Stage-A**: Introduction of `Driver.UpdateMachine` and supporting one-time post-startup configuration/adjustment of the instance. The [triggerCreationFlow](https://github.com/gardener/machine-controller-manager/blob/rel-v0.50/pkg/util/provider/machinecontroller/machine.go#L310) - a reconciliation sub-flow of the MCM responsible for orchestrating instance startup and updating machine status - will be updated to invoke `Driver.UpdateMachine` after `Driver.CreateMachine`

    _NOTE_: There is a legitimate argument to be made that `Driver.UpdateMachine` should be restricted to hot-updatable fields only and  post-startup initialization activities should be handled within the provider implementation of `CreateMachine`. Unfortunately, our `triggerCreationFlow` in the MCM 

2. **Stage-B**: Enhancing the MCM, MCM providers and gardener extension providers to support hot update of instances through `Driver.UpdateMachine`.  The MCM[triggerUpdationFlow](https://github.com/gardener/machine-controller-manager/blob/v0.50.1/pkg/util/provider/machinecontroller/machine.go#L531) - a reconciliation sub-flow of the MCM which is supposed to be responsible for orchestrating instance update - but currently not used, will be updated to invoke the provider `Driver.UpdateMachine` on hot-updates to to the `Machine` object

## Stage-A Proposal


### Enhancement of Driver Interface

We propose the following signature for the new `UpdateMachine` method.
```go
type Driver interface {
    // .. existing methods are omitted for brevity.

    // UpdateMachine call is responsible for updation of provider instance using the data in the given UpdateMachineRequest.
    UpdateMachine(context.Context, *UpdateMachineRequest) error
}

// UpdateMachineRequest is the request encapsulating identification and updation data regarding instance configuration. 
type UpdateMachineRequest struct {
    ProviderID string
    LastAppliedProviderSpec raw.Extension
    MachineClass *v1alpha1.MachineClass
    Secret *corev1.Secret
}
```
### Instance Not Ready Taint 

- Due to the fact that creation flow for machines has now a post startup update, we should not scheduled workload until the post startup udpate is done. (`Driver.CreateMachine`->`Driver.UpdateMachine`)
- We propose a new taint `node.machine.sapcloud.io/instance-not-ready` which is added as a node startup taint.
- The will will then removed by MCM, once the machine becomes fully ready. 
- We will add this taint as part of `--ignore-taint` in CA
- We will introduce a disclaimer / prerequisite in the MCM FAQ, to add this taint as part of kubelet config under `--register-with-taints`, otherwise workload could get scheduled , before machine beomes `Running`
- Q, NOTE: Unclear on where to contribute g/g changes ??


### Current MCM triggerCreationFlow

Today, [reconcileClusterMachine](https://github.com/gardener/machine-controller-manager/blob/v0.50.1/pkg/util/provider/machinecontroller/machine.go#L89) which is the main routine for the `Machine` object reconciliation invokes  [triggerCreationFlow](https://github.com/gardener/machine-controller-manager/blob/rel-v0.50/pkg/util/provider/machinecontroller/machine.go#L310) at the end when the `machine.Spec.ProviderID` is empty or if the `machine.Status.CurrentStatus.Phase` is empty or in `CrashLoopBackOff`

```mermaid
%%{ init: {
    'themeVariables':
        { 'fontSize': '10px'}
} }%%
flowchart LR

other["..."]
-->chk{"machine ProviderID empty
OR
Phase empty or CrashLoopBackOff ?
"}--yes-->triggerCreationFlow
chk--noo-->LongRetry["return machineutils.LongRetry"]
```

Today, the `triggerCreationFlow` looks like the below. 

* some minor details omitted/compressed for brevity 
* The `lastop` below is an abbreviation for `machine.Status.LastOperation`. This, along with the machine phase is generally updated on the `Machine` object just before returning from the method.
* `machineCreateErrorHandler` is a separate method on the controller that handles 
* regarding `phase=CrashLoopBackOff|Failed`. the machine phase may either be `CrashLoopBackOff` or move to `Failed` if the difference between current time  and the `machine.CreationTimestamp` has exceeded the configured `MachineCreationTimeout`.
 

```mermaid
%%{ init: {
    'themeVariables':
        { 'fontSize': '11px'}
} }%%
flowchart TD


end1(("end"))
begin((" "))
medretry["return MediumRetry, err"]
shortretry["return ShortRetry, err"]
medretry-->end1
shortretry-->end1

begin-->AddBootstrapTokenToUserData
-->gms["statusResp,statusErr=driver.GetMachineStatus(...)"]
-->chkstatuserr{"Checkk statusErr"}
chkstatuserr--notFound-->chknodelbl{"Chk Node Label"}
chkstatuserr--else-->createFailed["lastop.Type=Create,lastop.state=Failed,phase=CrashLoopBackOff|Failed"]-->medretry
chkstatuserr--nil-->initnodename["nodeName = statusResp.NodeName"]-->setnodename


chknodelbl--notset-->createmachine["createResp, createErr=driver.CreateMachine(...)"]-->chkCreateErr{"Check createErr"}

chkCreateErr--notnil-->createFailed

chkCreateErr--nil-->getnodename["nodeName = createResp.NodeName"]
-->chkstalenode{"nodeName != machine.Name\n//chk stale node"}
chkstalenode--false-->setnodename["if unset machine.Labels['node']= nodeName"]
-->machinepending["if empty/crashloopbackoff lastop.type=Create,lastop.State=Processing,phase=Pending"]
-->shortretry

chkstalenode--true-->delmachine["driver.DeleteMachine(...)"]
-->permafail["lastop.type=Create,lastop.state=Failed,Phase=Failed"]
-->shortretry

subgraph noteA [" "]
    permafail -.- note1(["VM was referring to stale node obj"])
end
style noteA opacity:0

subgraph noteB [" "]
    machinepending -.- note2(["Proposal: call to Driver.UpdateMachine in this section"])
end

```

### Enhancement of MCM triggerCreationFlow

(NOTE: WILL DRAW REVISED FLOWCHART FOR triggerCreationFlow ONLY AFTER CLARIFICATION)

1. We need to introduce a call to `Driver.UpdateMachine` in the flow illustrated above.
1. Observe that after the call to a successful `Driver.CreateMachine`, the machine phase is set to `Pending`, the `LastOperation.Type` is set to `Create` and the `LastOperation.State` set to `Processing`before returning with a `ShortRetry`. 
1. After interrogating the machine status (`Driver.GetMachineStatus`) and obtaining the `statusResp`, we can introduce the call do `Driver.CreateMachine`.
   1. We only do this invocation if  `Phase:Pending, LastOperation.Type:Create, LastOperation.State: Processing` OR if `Phase:Pending, LastOperation.Type:Update,LastOperation.State: Failed`
   1. After invocation of a successful call to `Driver.UpdateMachine`, we will update the machine object with `Phase: Pending, LastOperation.Type:Update, LastOperation.State: Successful`
      1. NOTE: This is a bit disharmonious, since in the earlier cycle the `LastOperation.State` was set to `Processing` after a successful call to `Driver.CreateMachine`. Q: What should be the right operation state ?
   1. After invocation of a failed call to `Driver.UpdateMachine` we will update the machine object with `Phase: Pending, LastOperation.Type:Update, LastOperation.State: Failed`


### Driver.UpdateMachine Implementation

#### AWS Provider Changes
The sequence of activities for the AWS Provider will look something like:
1. Check `providerSpec.SrcAndDstChecksEnabled`, construct `ModifyInstanceAttributeInput` and call `ModifyInstanceAttribute`
1. Check `providerSpec.NetworkInterfaces` and if `Ipv6PrefixCount` is not `nil`, then construct `AssignIpv6AddressesInput` and call `AssignIpv6Addresses` 

NOTE: Do I modify the [existing PR](https://github.com/gardener/machine-controller-manager-provider-aws/pull/128) for this ? 


#### TODO: Azure Provider
#### TODO: Other Providers




