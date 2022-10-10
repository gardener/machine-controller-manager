## Specification
### ProviderSpec Schema
<br>
<h3 id="AWSMachineClass">
<b>AWSMachineClass</b>
</h3>
<p>
<p>AWSMachineClass TODO</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code>
</td>
<td>
string
</td>
<td>
<code>
machine.sapcloud.io.v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code>
</td>
<td>
string
</td>
<td>
<code>AWSMachineClass</code>
</td>
</tr>
<tr>
<td>
<code>metadata</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code>
</td>
<td>
<em>
<a href="#AWSMachineClassSpec">
AWSMachineClassSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<br/>
<br/>
<table>
<tr>
<td>
<code>ami</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>region</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>blockDevices</code>
</td>
<td>
<em>
<a href="#AWSBlockDeviceMappingSpec">
[]AWSBlockDeviceMappingSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>ebsOptimized</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>iam</code>
</td>
<td>
<em>
<a href="#AWSIAMProfileSpec">
AWSIAMProfileSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>machineType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>keyName</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>monitoring</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>networkInterfaces</code>
</td>
<td>
<em>
<a href="#AWSNetworkInterfaceSpec">
[]AWSNetworkInterfaceSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tags</code>
</td>
<td>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>spotPrice</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>credentialsSecretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AlicloudMachineClass">
<b>AlicloudMachineClass</b>
</h3>
<p>
<p>AlicloudMachineClass TODO</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code>
</td>
<td>
string
</td>
<td>
<code>
machine.sapcloud.io.v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code>
</td>
<td>
string
</td>
<td>
<code>AlicloudMachineClass</code>
</td>
</tr>
<tr>
<td>
<code>metadata</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code>
</td>
<td>
<em>
<a href="#AlicloudMachineClassSpec">
AlicloudMachineClassSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<br/>
<br/>
<table>
<tr>
<td>
<code>imageID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>instanceType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>region</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>zoneID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>securityGroupID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>vSwitchID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>privateIPAddress</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>systemDisk</code>
</td>
<td>
<em>
<a href="#AlicloudSystemDisk">
AlicloudSystemDisk
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>dataDisks</code>
</td>
<td>
<em>
<a href="#AlicloudDataDisk">
[]AlicloudDataDisk
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>instanceChargeType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>internetChargeType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>internetMaxBandwidthIn</code>
</td>
<td>
<em>
*int
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>internetMaxBandwidthOut</code>
</td>
<td>
<em>
*int
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>spotStrategy</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>IoOptimized</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tags</code>
</td>
<td>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>keyPairName</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>credentialsSecretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureMachineClass">
<b>AzureMachineClass</b>
</h3>
<p>
<p>AzureMachineClass TODO</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code>
</td>
<td>
string
</td>
<td>
<code>
machine.sapcloud.io.v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code>
</td>
<td>
string
</td>
<td>
<code>AzureMachineClass</code>
</td>
</tr>
<tr>
<td>
<code>metadata</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code>
</td>
<td>
<em>
<a href="#AzureMachineClassSpec">
AzureMachineClassSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<br/>
<br/>
<table>
<tr>
<td>
<code>location</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tags</code>
</td>
<td>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>properties</code>
</td>
<td>
<em>
<a href="#AzureVirtualMachineProperties">
AzureVirtualMachineProperties
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>resourceGroup</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>subnetInfo</code>
</td>
<td>
<em>
<a href="#AzureSubnetInfo">
AzureSubnetInfo
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>credentialsSecretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="GCPMachineClass">
<b>GCPMachineClass</b>
</h3>
<p>
<p>GCPMachineClass TODO</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code>
</td>
<td>
string
</td>
<td>
<code>
machine.sapcloud.io.v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code>
</td>
<td>
string
</td>
<td>
<code>GCPMachineClass</code>
</td>
</tr>
<tr>
<td>
<code>metadata</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code>
</td>
<td>
<em>
<a href="#GCPMachineClassSpec">
GCPMachineClassSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<br/>
<br/>
<table>
<tr>
<td>
<code>canIpForward</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>deletionProtection</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>description</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>disks</code>
</td>
<td>
<em>
<a href="#%2agithub.com%2fgardener%2fmachine-controller-manager%2fpkg%2fapis%2fmachine%2fv1alpha1.GCPDisk">
[]*github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1.GCPDisk
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>labels</code>
</td>
<td>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>machineType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>metadata</code>
</td>
<td>
<em>
<a href="#%2agithub.com%2fgardener%2fmachine-controller-manager%2fpkg%2fapis%2fmachine%2fv1alpha1.GCPMetadata">
[]*github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1.GCPMetadata
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>networkInterfaces</code>
</td>
<td>
<em>
<a href="#%2agithub.com%2fgardener%2fmachine-controller-manager%2fpkg%2fapis%2fmachine%2fv1alpha1.GCPNetworkInterface">
[]*github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1.GCPNetworkInterface
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>scheduling</code>
</td>
<td>
<em>
<a href="#GCPScheduling">
GCPScheduling
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>credentialsSecretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>serviceAccounts</code>
</td>
<td>
<em>
<a href="#GCPServiceAccount">
[]GCPServiceAccount
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tags</code>
</td>
<td>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>region</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>zone</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="Machine">
<b>Machine</b>
</h3>
<p>
<p>Machine is the representation of a physical or virtual machine.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code>
</td>
<td>
string
</td>
<td>
<code>
machine.sapcloud.io.v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code>
</td>
<td>
string
</td>
<td>
<code>Machine</code>
</td>
</tr>
<tr>
<td>
<code>metadata</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<p>ObjectMeta for machine object</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code>
</td>
<td>
<em>
<a href="#MachineSpec">
MachineSpec
</a>
</em>
</td>
<td>
<p>Spec contains the specification of the machine</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>class</code>
</td>
<td>
<em>
<a href="#ClassSpec">
ClassSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class contains the machineclass attributes of a machine</p>
</td>
</tr>
<tr>
<td>
<code>providerID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderID represents the provider&rsquo;s unique ID given to a machine</p>
</td>
</tr>
<tr>
<td>
<code>nodeTemplate</code>
</td>
<td>
<em>
<a href="#NodeTemplateSpec">
NodeTemplateSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeTemplateSpec describes the data a node should have when created from a template</p>
</td>
</tr>
<tr>
<td>
<code>MachineConfiguration</code>
</td>
<td>
<em>
<a href="#MachineConfiguration">
MachineConfiguration
</a>
</em>
</td>
<td>
<p>
(Members of <code>MachineConfiguration</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>Configuration for the machine-controller.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code>
</td>
<td>
<em>
<a href="#MachineStatus">
MachineStatus
</a>
</em>
</td>
<td>
<p>Status contains fields depicting the status</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="MachineClass">
<b>MachineClass</b>
</h3>
<p>
<p>MachineClass can be used to templatize and re-use provider configuration
across multiple Machines / MachineSets / MachineDeployments.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code>
</td>
<td>
string
</td>
<td>
<code>
machine.sapcloud.io.v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code>
</td>
<td>
string
</td>
<td>
<code>MachineClass</code>
</td>
</tr>
<tr>
<td>
<code>metadata</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>nodeTemplate</code>
</td>
<td>
<em>
<a href="#NodeTemplate">
NodeTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeTemplate contains subfields to track all node resources and other node info required to scale nodegroup from zero</p>
</td>
</tr>
<tr>
<td>
<code>credentialsSecretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<p>CredentialsSecretRef can optionally store the credentials (in this case the SecretRef does not need to store them).
This might be useful if multiple machine classes with the same credentials but different user-datas are used.</p>
</td>
</tr>
<tr>
<td>
<code>providerSpec</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fgodoc.org%2fk8s.io%2fapimachinery%2fpkg%2fruntime%23RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
</em>
</td>
<td>
<p>Provider-specific configuration to use during node creation.</p>
</td>
</tr>
<tr>
<td>
<code>provider</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>Provider is the combination of name and location of cloud-specific drivers.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<p>SecretRef stores the necessary secrets such as credentials or userdata.</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="MachineDeployment">
<b>MachineDeployment</b>
</h3>
<p>
<p>MachineDeployment enables declarative updates for machines and MachineSets.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code>
</td>
<td>
string
</td>
<td>
<code>
machine.sapcloud.io.v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code>
</td>
<td>
string
</td>
<td>
<code>MachineDeployment</code>
</td>
</tr>
<tr>
<td>
<code>metadata</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Standard object metadata.</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code>
</td>
<td>
<em>
<a href="#MachineDeploymentSpec">
MachineDeploymentSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the desired behavior of the MachineDeployment.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>replicas</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Number of desired machines. This is a pointer to distinguish between explicit
zero and not specified. Defaults to 0.</p>
</td>
</tr>
<tr>
<td>
<code>selector</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Label selector for machines. Existing MachineSets whose machines are
selected by this will be the ones affected by this MachineDeployment.</p>
</td>
</tr>
<tr>
<td>
<code>template</code>
</td>
<td>
<em>
<a href="#MachineTemplateSpec">
MachineTemplateSpec
</a>
</em>
</td>
<td>
<p>Template describes the machines that will be created.</p>
</td>
</tr>
<tr>
<td>
<code>strategy</code>
</td>
<td>
<em>
<a href="#MachineDeploymentStrategy">
MachineDeploymentStrategy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The MachineDeployment strategy to use to replace existing machines with new ones.</p>
</td>
</tr>
<tr>
<td>
<code>minReadySeconds</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Minimum number of seconds for which a newly created machine should be ready
without any of its container crashing, for it to be considered available.
Defaults to 0 (machine will be considered available as soon as it is ready)</p>
</td>
</tr>
<tr>
<td>
<code>revisionHistoryLimit</code>
</td>
<td>
<em>
*int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>The number of old MachineSets to retain to allow rollback.
This is a pointer to distinguish between explicit zero and not specified.</p>
</td>
</tr>
<tr>
<td>
<code>paused</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates that the MachineDeployment is paused and will not be processed by the
MachineDeployment controller.</p>
</td>
</tr>
<tr>
<td>
<code>rollbackTo</code>
</td>
<td>
<em>
<a href="#RollbackConfig">
RollbackConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DEPRECATED.
The config this MachineDeployment is rolling back to. Will be cleared after rollback is done.</p>
</td>
</tr>
<tr>
<td>
<code>progressDeadlineSeconds</code>
</td>
<td>
<em>
*int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum time in seconds for a MachineDeployment to make progress before it
is considered to be failed. The MachineDeployment controller will continue to
process failed MachineDeployments and a condition with a ProgressDeadlineExceeded
reason will be surfaced in the MachineDeployment status. Note that progress will
not be estimated during the time a MachineDeployment is paused. This is not set
by default.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code>
</td>
<td>
<em>
<a href="#MachineDeploymentStatus">
MachineDeploymentStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Most recently observed status of the MachineDeployment.</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="MachineSet">
<b>MachineSet</b>
</h3>
<p>
<p>MachineSet TODO</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code>
</td>
<td>
string
</td>
<td>
<code>
machine.sapcloud.io.v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code>
</td>
<td>
string
</td>
<td>
<code>MachineSet</code>
</td>
</tr>
<tr>
<td>
<code>metadata</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code>
</td>
<td>
<em>
<a href="#MachineSetSpec">
MachineSetSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<br/>
<br/>
<table>
<tr>
<td>
<code>replicas</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>selector</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>machineClass</code>
</td>
<td>
<em>
<a href="#ClassSpec">
ClassSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>template</code>
</td>
<td>
<em>
<a href="#MachineTemplateSpec">
MachineTemplateSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>minReadySeconds</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code>
</td>
<td>
<em>
<a href="#MachineSetStatus">
MachineSetStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="PacketMachineClass">
<b>PacketMachineClass</b>
</h3>
<p>
<p>PacketMachineClass TODO</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code>
</td>
<td>
string
</td>
<td>
<code>
machine.sapcloud.io.v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code>
</td>
<td>
string
</td>
<td>
<code>PacketMachineClass</code>
</td>
</tr>
<tr>
<td>
<code>metadata</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code>
</td>
<td>
<em>
<a href="#PacketMachineClassSpec">
PacketMachineClassSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<br/>
<br/>
<table>
<tr>
<td>
<code>facility</code>
</td>
<td>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>machineType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>billingCycle</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>OS</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>projectID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tags</code>
</td>
<td>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>sshKeys</code>
</td>
<td>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>userdata</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>credentialsSecretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AWSBlockDeviceMappingSpec">
<b>AWSBlockDeviceMappingSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AWSMachineClassSpec">AWSMachineClassSpec</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>deviceName</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>The device name exposed to the machine (for example, /dev/sdh or xvdh).</p>
</td>
</tr>
<tr>
<td>
<code>ebs</code>
</td>
<td>
<em>
<a href="#AWSEbsBlockDeviceSpec">
AWSEbsBlockDeviceSpec
</a>
</em>
</td>
<td>
<p>Parameters used to automatically set up EBS volumes when the machine is
launched.</p>
</td>
</tr>
<tr>
<td>
<code>noDevice</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>Suppresses the specified device included in the block device mapping of the
AMI.</p>
</td>
</tr>
<tr>
<td>
<code>virtualName</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>The virtual device name (ephemeralN). Machine store volumes are numbered
starting from 0. An machine type with 2 available machine store volumes
can specify mappings for ephemeral0 and ephemeral1.The number of available
machine store volumes depends on the machine type. After you connect to
the machine, you must mount the volume.</p>
<p>Constraints: For M3 machines, you must specify machine store volumes in
the block device mapping for the machine. When you launch an M3 machine,
we ignore any machine store volumes specified in the block device mapping
for the AMI.</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AWSEbsBlockDeviceSpec">
<b>AWSEbsBlockDeviceSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AWSBlockDeviceMappingSpec">AWSBlockDeviceMappingSpec</a>)
</p>
<p>
<p>Describes a block device for an EBS volume.
Please also see <a href="https://docs.aws.amazon.com/goto/WebAPI/ec2-2016-11-15/EbsBlockDevice">https://docs.aws.amazon.com/goto/WebAPI/ec2-2016-11-15/EbsBlockDevice</a></p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>deleteOnTermination</code>
</td>
<td>
<em>
*bool
</em>
</td>
<td>
<p>Indicates whether the EBS volume is deleted on machine termination.</p>
</td>
</tr>
<tr>
<td>
<code>encrypted</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
<p>Indicates whether the EBS volume is encrypted. Encrypted Amazon EBS volumes
may only be attached to machines that support Amazon EBS encryption.</p>
</td>
</tr>
<tr>
<td>
<code>iops</code>
</td>
<td>
<em>
int64
</em>
</td>
<td>
<p>The number of I/O operations per second (IOPS) that the volume supports.
For io1, this represents the number of IOPS that are provisioned for the
volume. For gp2, this represents the baseline performance of the volume and
the rate at which the volume accumulates I/O credits for bursting. For more
information about General Purpose SSD baseline performance, I/O credits,
and bursting, see Amazon EBS Volume Types (<a href="http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html">http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html</a>)
in the Amazon Elastic Compute Cloud User Guide.</p>
<p>Constraint: Range is 100-20000 IOPS for io1 volumes and 100-10000 IOPS for
gp2 volumes.</p>
<p>Condition: This parameter is required for requests to create io1 volumes;
it is not used in requests to create gp2, st1, sc1, or standard volumes.</p>
</td>
</tr>
<tr>
<td>
<code>kmsKeyID</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
<p>Identifier (key ID, key alias, ID ARN, or alias ARN) for a customer managed
CMK under which the EBS volume is encrypted.</p>
<p>This parameter is only supported on BlockDeviceMapping objects called by
RunInstances (<a href="https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RunInstances.html">https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RunInstances.html</a>),
RequestSpotFleet (<a href="https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RequestSpotFleet.html">https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RequestSpotFleet.html</a>),
and RequestSpotInstances (<a href="https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RequestSpotInstances.html">https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RequestSpotInstances.html</a>).</p>
</td>
</tr>
<tr>
<td>
<code>snapshotID</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
<p>The ID of the snapshot.</p>
</td>
</tr>
<tr>
<td>
<code>volumeSize</code>
</td>
<td>
<em>
int64
</em>
</td>
<td>
<p>The size of the volume, in GiB.</p>
<p>Constraints: 1-16384 for General Purpose SSD (gp2), 4-16384 for Provisioned
IOPS SSD (io1), 500-16384 for Throughput Optimized HDD (st1), 500-16384 for
Cold HDD (sc1), and 1-1024 for Magnetic (standard) volumes. If you specify
a snapshot, the volume size must be equal to or larger than the snapshot
size.</p>
<p>Default: If you&rsquo;re creating the volume from a snapshot and don&rsquo;t specify
a volume size, the default is the snapshot size.</p>
</td>
</tr>
<tr>
<td>
<code>volumeType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>The volume type: gp2, io1, st1, sc1, or standard.</p>
<p>Default: standard</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AWSIAMProfileSpec">
<b>AWSIAMProfileSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AWSMachineClassSpec">AWSMachineClassSpec</a>)
</p>
<p>
<p>Describes an IAM machine profile.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>arn</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>The Amazon Resource Name (ARN) of the machine profile.</p>
</td>
</tr>
<tr>
<td>
<code>name</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>The name of the machine profile.</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AWSMachineClassSpec">
<b>AWSMachineClassSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AWSMachineClass">AWSMachineClass</a>)
</p>
<p>
<p>AWSMachineClassSpec is the specification of a AWSMachineClass.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ami</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>region</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>blockDevices</code>
</td>
<td>
<em>
<a href="#AWSBlockDeviceMappingSpec">
[]AWSBlockDeviceMappingSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>ebsOptimized</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>iam</code>
</td>
<td>
<em>
<a href="#AWSIAMProfileSpec">
AWSIAMProfileSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>machineType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>keyName</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>monitoring</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>networkInterfaces</code>
</td>
<td>
<em>
<a href="#AWSNetworkInterfaceSpec">
[]AWSNetworkInterfaceSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tags</code>
</td>
<td>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>spotPrice</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>credentialsSecretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AWSNetworkInterfaceSpec">
<b>AWSNetworkInterfaceSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AWSMachineClassSpec">AWSMachineClassSpec</a>)
</p>
<p>
<p>Describes a network interface.
Please also see <a href="https://docs.aws.amazon.com/goto/WebAPI/ec2-2016-11-15/MachineAWSNetworkInterfaceSpecification">https://docs.aws.amazon.com/goto/WebAPI/ec2-2016-11-15/MachineAWSNetworkInterfaceSpecification</a></p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>associatePublicIPAddress</code>
</td>
<td>
<em>
*bool
</em>
</td>
<td>
<p>Indicates whether to assign a public IPv4 address to an machine you launch
in a VPC. The public IP address can only be assigned to a network interface
for eth0, and can only be assigned to a new network interface, not an existing
one. You cannot specify more than one network interface in the request. If
launching into a default subnet, the default value is true.</p>
</td>
</tr>
<tr>
<td>
<code>deleteOnTermination</code>
</td>
<td>
<em>
*bool
</em>
</td>
<td>
<p>If set to true, the interface is deleted when the machine is terminated.
You can specify true only if creating a new network interface when launching
an machine.</p>
</td>
</tr>
<tr>
<td>
<code>description</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
<p>The description of the network interface. Applies only if creating a network
interface when launching an machine.</p>
</td>
</tr>
<tr>
<td>
<code>securityGroupIDs</code>
</td>
<td>
<em>
[]string
</em>
</td>
<td>
<p>The IDs of the security groups for the network interface. Applies only if
creating a network interface when launching an machine.</p>
</td>
</tr>
<tr>
<td>
<code>subnetID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>The ID of the subnet associated with the network string. Applies only if
creating a network interface when launching an machine.</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AlicloudDataDisk">
<b>AlicloudDataDisk</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AlicloudMachineClassSpec">AlicloudMachineClassSpec</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>category</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>description</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>encrypted</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>deleteWithInstance</code>
</td>
<td>
<em>
*bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>size</code>
</td>
<td>
<em>
int
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AlicloudMachineClassSpec">
<b>AlicloudMachineClassSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AlicloudMachineClass">AlicloudMachineClass</a>)
</p>
<p>
<p>AlicloudMachineClassSpec is the specification of a AlicloudMachineClass.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>imageID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>instanceType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>region</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>zoneID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>securityGroupID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>vSwitchID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>privateIPAddress</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>systemDisk</code>
</td>
<td>
<em>
<a href="#AlicloudSystemDisk">
AlicloudSystemDisk
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>dataDisks</code>
</td>
<td>
<em>
<a href="#AlicloudDataDisk">
[]AlicloudDataDisk
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>instanceChargeType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>internetChargeType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>internetMaxBandwidthIn</code>
</td>
<td>
<em>
*int
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>internetMaxBandwidthOut</code>
</td>
<td>
<em>
*int
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>spotStrategy</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>IoOptimized</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tags</code>
</td>
<td>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>keyPairName</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>credentialsSecretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AlicloudSystemDisk">
<b>AlicloudSystemDisk</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AlicloudMachineClassSpec">AlicloudMachineClassSpec</a>)
</p>
<p>
<p>AlicloudSystemDisk describes SystemDisk for Alicloud.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>category</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>size</code>
</td>
<td>
<em>
int
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureDataDisk">
<b>AzureDataDisk</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureStorageProfile">AzureStorageProfile</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>lun</code>
</td>
<td>
<em>
*int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>caching</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>storageAccountType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>diskSizeGB</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureHardwareProfile">
<b>AzureHardwareProfile</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureVirtualMachineProperties">AzureVirtualMachineProperties</a>)
</p>
<p>
<p>AzureHardwareProfile is specifies the hardware settings for the virtual machine.
Refer github.com/Azure/azure-sdk-for-go/arm/compute/models.go for VMSizes</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>vmSize</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureImageReference">
<b>AzureImageReference</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureStorageProfile">AzureStorageProfile</a>)
</p>
<p>
<p>AzureImageReference is specifies information about the image to use. You can specify information about platform images,
marketplace images, or virtual machine images. This element is required when you want to use a platform image,
marketplace image, or virtual machine image, but is not used in other creation operations.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>id</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>urn</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
<p>Uniform Resource Name of the OS image to be used , it has the format &lsquo;publisher:offer:sku:version&rsquo;</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureLinuxConfiguration">
<b>AzureLinuxConfiguration</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureOSProfile">AzureOSProfile</a>)
</p>
<p>
<p>AzureLinuxConfiguration is specifies the Linux operating system settings on the virtual machine. <br><br>For a list of
supported Linux distributions, see <a href="https://docs.microsoft.com/azure/virtual-machines/virtual-machines-linux-endorsed-distros?toc=%2fazure%2fvirtual-machines%2flinux%2ftoc.json">Linux on Azure-Endorsed
Distributions</a>
<br><br> For running non-endorsed distributions, see <a href="https://docs.microsoft.com/azure/virtual-machines/virtual-machines-linux-create-upload-generic?toc=%2fazure%2fvirtual-machines%2flinux%2ftoc.json">Information for Non-Endorsed
Distributions</a>.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>disablePasswordAuthentication</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>ssh</code>
</td>
<td>
<em>
<a href="#AzureSSHConfiguration">
AzureSSHConfiguration
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureMachineClassSpec">
<b>AzureMachineClassSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureMachineClass">AzureMachineClass</a>)
</p>
<p>
<p>AzureMachineClassSpec is the specification of a AzureMachineClass.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>location</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tags</code>
</td>
<td>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>properties</code>
</td>
<td>
<em>
<a href="#AzureVirtualMachineProperties">
AzureVirtualMachineProperties
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>resourceGroup</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>subnetInfo</code>
</td>
<td>
<em>
<a href="#AzureSubnetInfo">
AzureSubnetInfo
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>credentialsSecretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureMachineSetConfig">
<b>AzureMachineSetConfig</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureVirtualMachineProperties">AzureVirtualMachineProperties</a>)
</p>
<p>
<p>AzureMachineSetConfig contains the information about the machine set</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>id</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>kind</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureManagedDiskParameters">
<b>AzureManagedDiskParameters</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureOSDisk">AzureOSDisk</a>)
</p>
<p>
<p>AzureManagedDiskParameters is the parameters of a managed disk.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>id</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>storageAccountType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureNetworkInterfaceReference">
<b>AzureNetworkInterfaceReference</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureNetworkProfile">AzureNetworkProfile</a>)
</p>
<p>
<p>AzureNetworkInterfaceReference is describes a network interface reference.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>id</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>properties</code>
</td>
<td>
<em>
<a href="#AzureNetworkInterfaceReferenceProperties">
AzureNetworkInterfaceReferenceProperties
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureNetworkInterfaceReferenceProperties">
<b>AzureNetworkInterfaceReferenceProperties</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureNetworkInterfaceReference">AzureNetworkInterfaceReference</a>)
</p>
<p>
<p>AzureNetworkInterfaceReferenceProperties is describes a network interface reference properties.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>primary</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureNetworkProfile">
<b>AzureNetworkProfile</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureVirtualMachineProperties">AzureVirtualMachineProperties</a>)
</p>
<p>
<p>AzureNetworkProfile is specifies the network interfaces of the virtual machine.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>networkInterfaces</code>
</td>
<td>
<em>
<a href="#AzureNetworkInterfaceReference">
AzureNetworkInterfaceReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>acceleratedNetworking</code>
</td>
<td>
<em>
*bool
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureOSDisk">
<b>AzureOSDisk</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureStorageProfile">AzureStorageProfile</a>)
</p>
<p>
<p>AzureOSDisk is specifies information about the operating system disk used by the virtual machine. <br><br> For more
information about disks, see <a href="https://docs.microsoft.com/azure/virtual-machines/virtual-machines-windows-about-disks-vhds?toc=%2fazure%2fvirtual-machines%2fwindows%2ftoc.json">About disks and VHDs for Azure virtual
machines</a>.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>caching</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>managedDisk</code>
</td>
<td>
<em>
<a href="#AzureManagedDiskParameters">
AzureManagedDiskParameters
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>diskSizeGB</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>createOption</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureOSProfile">
<b>AzureOSProfile</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureVirtualMachineProperties">AzureVirtualMachineProperties</a>)
</p>
<p>
<p>AzureOSProfile is specifies the operating system settings for the virtual machine.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>computerName</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>adminUsername</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>adminPassword</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>customData</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>linuxConfiguration</code>
</td>
<td>
<em>
<a href="#AzureLinuxConfiguration">
AzureLinuxConfiguration
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureSSHConfiguration">
<b>AzureSSHConfiguration</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureLinuxConfiguration">AzureLinuxConfiguration</a>)
</p>
<p>
<p>AzureSSHConfiguration is SSH configuration for Linux based VMs running on Azure</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>publicKeys</code>
</td>
<td>
<em>
<a href="#AzureSSHPublicKey">
AzureSSHPublicKey
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureSSHPublicKey">
<b>AzureSSHPublicKey</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureSSHConfiguration">AzureSSHConfiguration</a>)
</p>
<p>
<p>AzureSSHPublicKey is contains information about SSH certificate public key and the path on the Linux VM where the public
key is placed.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>path</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>keyData</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureStorageProfile">
<b>AzureStorageProfile</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureVirtualMachineProperties">AzureVirtualMachineProperties</a>)
</p>
<p>
<p>AzureStorageProfile is specifies the storage settings for the virtual machine disks.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>imageReference</code>
</td>
<td>
<em>
<a href="#AzureImageReference">
AzureImageReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>osDisk</code>
</td>
<td>
<em>
<a href="#AzureOSDisk">
AzureOSDisk
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>dataDisks</code>
</td>
<td>
<em>
<a href="#AzureDataDisk">
[]AzureDataDisk
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureSubResource">
<b>AzureSubResource</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureVirtualMachineProperties">AzureVirtualMachineProperties</a>)
</p>
<p>
<p>AzureSubResource is the Sub Resource definition.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>id</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureSubnetInfo">
<b>AzureSubnetInfo</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureMachineClassSpec">AzureMachineClassSpec</a>)
</p>
<p>
<p>AzureSubnetInfo is the information containing the subnet details</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>vnetName</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>vnetResourceGroup</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>subnetName</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="AzureVirtualMachineProperties">
<b>AzureVirtualMachineProperties</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#AzureMachineClassSpec">AzureMachineClassSpec</a>)
</p>
<p>
<p>AzureVirtualMachineProperties is describes the properties of a Virtual Machine.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>hardwareProfile</code>
</td>
<td>
<em>
<a href="#AzureHardwareProfile">
AzureHardwareProfile
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>storageProfile</code>
</td>
<td>
<em>
<a href="#AzureStorageProfile">
AzureStorageProfile
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>osProfile</code>
</td>
<td>
<em>
<a href="#AzureOSProfile">
AzureOSProfile
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>networkProfile</code>
</td>
<td>
<em>
<a href="#AzureNetworkProfile">
AzureNetworkProfile
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>availabilitySet</code>
</td>
<td>
<em>
<a href="#AzureSubResource">
AzureSubResource
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>identityID</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>zone</code>
</td>
<td>
<em>
*int
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>machineSet</code>
</td>
<td>
<em>
<a href="#AzureMachineSetConfig">
AzureMachineSetConfig
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="ClassSpec">
<b>ClassSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineSetSpec">MachineSetSpec</a>, 
<a href="#MachineSpec">MachineSpec</a>)
</p>
<p>
<p>ClassSpec is the class specification of machine</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiGroup</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>API group to which it belongs</p>
</td>
</tr>
<tr>
<td>
<code>kind</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>Kind for machine class</p>
</td>
</tr>
<tr>
<td>
<code>name</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>Name of machine class</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="ConditionStatus">
<b>ConditionStatus</b>
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#MachineDeploymentCondition">MachineDeploymentCondition</a>, 
<a href="#MachineSetCondition">MachineSetCondition</a>)
</p>
<p>
</p>
<br>
<h3 id="CurrentStatus">
<b>CurrentStatus</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineStatus">MachineStatus</a>)
</p>
<p>
<p>CurrentStatus contains information about the current status of Machine.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>phase</code>
</td>
<td>
<em>
<a href="#MachinePhase">
MachinePhase
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>timeoutActive</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>lastUpdateTime</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>Last update time of current status</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="GCPDisk">
<b>GCPDisk</b>
</h3>
<p>
<p>GCPDisk describes disks for GCP.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>autoDelete</code>
</td>
<td>
<em>
*bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>boot</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>sizeGb</code>
</td>
<td>
<em>
int64
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>type</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>interface</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>image</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>labels</code>
</td>
<td>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="GCPMachineClassSpec">
<b>GCPMachineClassSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#GCPMachineClass">GCPMachineClass</a>)
</p>
<p>
<p>GCPMachineClassSpec is the specification of a GCPMachineClass.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>canIpForward</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>deletionProtection</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>description</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>disks</code>
</td>
<td>
<em>
<a href="#%2agithub.com%2fgardener%2fmachine-controller-manager%2fpkg%2fapis%2fmachine%2fv1alpha1.GCPDisk">
[]*github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1.GCPDisk
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>labels</code>
</td>
<td>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>machineType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>metadata</code>
</td>
<td>
<em>
<a href="#%2agithub.com%2fgardener%2fmachine-controller-manager%2fpkg%2fapis%2fmachine%2fv1alpha1.GCPMetadata">
[]*github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1.GCPMetadata
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>networkInterfaces</code>
</td>
<td>
<em>
<a href="#%2agithub.com%2fgardener%2fmachine-controller-manager%2fpkg%2fapis%2fmachine%2fv1alpha1.GCPNetworkInterface">
[]*github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1.GCPNetworkInterface
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>scheduling</code>
</td>
<td>
<em>
<a href="#GCPScheduling">
GCPScheduling
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>credentialsSecretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>serviceAccounts</code>
</td>
<td>
<em>
<a href="#GCPServiceAccount">
[]GCPServiceAccount
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tags</code>
</td>
<td>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>region</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>zone</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="GCPMetadata">
<b>GCPMetadata</b>
</h3>
<p>
<p>GCPMetadata describes metadata for GCP.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>key</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>value</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="GCPNetworkInterface">
<b>GCPNetworkInterface</b>
</h3>
<p>
<p>GCPNetworkInterface describes network interfaces for GCP</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>disableExternalIP</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>network</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>subnetwork</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="GCPScheduling">
<b>GCPScheduling</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#GCPMachineClassSpec">GCPMachineClassSpec</a>)
</p>
<p>
<p>GCPScheduling describes scheduling configuration for GCP.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>automaticRestart</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>onHostMaintenance</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>preemptible</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="GCPServiceAccount">
<b>GCPServiceAccount</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#GCPMachineClassSpec">GCPMachineClassSpec</a>)
</p>
<p>
<p>GCPServiceAccount describes service accounts for GCP.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>email</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>scopes</code>
</td>
<td>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="LastOperation">
<b>LastOperation</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineSetStatus">MachineSetStatus</a>, 
<a href="#MachineStatus">MachineStatus</a>, 
<a href="#MachineSummary">MachineSummary</a>)
</p>
<p>
<p>LastOperation suggests the last operation performed on the object</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>description</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>Description of the current operation</p>
</td>
</tr>
<tr>
<td>
<code>lastUpdateTime</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>Last update time of current operation</p>
</td>
</tr>
<tr>
<td>
<code>state</code>
</td>
<td>
<em>
<a href="#MachineState">
MachineState
</a>
</em>
</td>
<td>
<p>State of operation</p>
</td>
</tr>
<tr>
<td>
<code>type</code>
</td>
<td>
<em>
<a href="#MachineOperationType">
MachineOperationType
</a>
</em>
</td>
<td>
<p>Type of operation</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="MachineConfiguration">
<b>MachineConfiguration</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineSpec">MachineSpec</a>)
</p>
<p>
<p>MachineConfiguration describes the configurations useful for the machine-controller.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>drainTimeout</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fgodoc.org%2fk8s.io%2fapimachinery%2fpkg%2fapis%2fmeta%2fv1%23Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineDraintimeout is the timeout after which machine is forcefully deleted.</p>
</td>
</tr>
<tr>
<td>
<code>healthTimeout</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fgodoc.org%2fk8s.io%2fapimachinery%2fpkg%2fapis%2fmeta%2fv1%23Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineHealthTimeout is the timeout after which machine is declared unhealhty/failed.</p>
</td>
</tr>
<tr>
<td>
<code>creationTimeout</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fgodoc.org%2fk8s.io%2fapimachinery%2fpkg%2fapis%2fmeta%2fv1%23Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineCreationTimeout is the timeout after which machinie creation is declared failed.</p>
</td>
</tr>
<tr>
<td>
<code>maxEvictRetries</code>
</td>
<td>
<em>
*int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxEvictRetries is the number of retries that will be attempted while draining the node.</p>
</td>
</tr>
<tr>
<td>
<code>nodeConditions</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeConditions are the set of conditions if set to true for MachineHealthTimeOut, machine will be declared failed.</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="MachineDeploymentCondition">
<b>MachineDeploymentCondition</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineDeploymentStatus">MachineDeploymentStatus</a>)
</p>
<p>
<p>MachineDeploymentCondition describes the state of a MachineDeployment at a certain point.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code>
</td>
<td>
<em>
<a href="#MachineDeploymentConditionType">
MachineDeploymentConditionType
</a>
</em>
</td>
<td>
<p>Type of MachineDeployment condition.</p>
</td>
</tr>
<tr>
<td>
<code>status</code>
</td>
<td>
<em>
<a href="#ConditionStatus">
ConditionStatus
</a>
</em>
</td>
<td>
<p>Status of the condition, one of True, False, Unknown.</p>
</td>
</tr>
<tr>
<td>
<code>lastUpdateTime</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>The last time this condition was updated.</p>
</td>
</tr>
<tr>
<td>
<code>lastTransitionTime</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>Last time the condition transitioned from one status to another.</p>
</td>
</tr>
<tr>
<td>
<code>reason</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>The reason for the condition&rsquo;s last transition.</p>
</td>
</tr>
<tr>
<td>
<code>message</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>A human readable message indicating details about the transition.</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="MachineDeploymentConditionType">
<b>MachineDeploymentConditionType</b>
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#MachineDeploymentCondition">MachineDeploymentCondition</a>)
</p>
<p>
</p>
<br>
<h3 id="MachineDeploymentSpec">
<b>MachineDeploymentSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineDeployment">MachineDeployment</a>)
</p>
<p>
<p>MachineDeploymentSpec is the specification of the desired behavior of the MachineDeployment.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>replicas</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Number of desired machines. This is a pointer to distinguish between explicit
zero and not specified. Defaults to 0.</p>
</td>
</tr>
<tr>
<td>
<code>selector</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Label selector for machines. Existing MachineSets whose machines are
selected by this will be the ones affected by this MachineDeployment.</p>
</td>
</tr>
<tr>
<td>
<code>template</code>
</td>
<td>
<em>
<a href="#MachineTemplateSpec">
MachineTemplateSpec
</a>
</em>
</td>
<td>
<p>Template describes the machines that will be created.</p>
</td>
</tr>
<tr>
<td>
<code>strategy</code>
</td>
<td>
<em>
<a href="#MachineDeploymentStrategy">
MachineDeploymentStrategy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The MachineDeployment strategy to use to replace existing machines with new ones.</p>
</td>
</tr>
<tr>
<td>
<code>minReadySeconds</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Minimum number of seconds for which a newly created machine should be ready
without any of its container crashing, for it to be considered available.
Defaults to 0 (machine will be considered available as soon as it is ready)</p>
</td>
</tr>
<tr>
<td>
<code>revisionHistoryLimit</code>
</td>
<td>
<em>
*int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>The number of old MachineSets to retain to allow rollback.
This is a pointer to distinguish between explicit zero and not specified.</p>
</td>
</tr>
<tr>
<td>
<code>paused</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates that the MachineDeployment is paused and will not be processed by the
MachineDeployment controller.</p>
</td>
</tr>
<tr>
<td>
<code>rollbackTo</code>
</td>
<td>
<em>
<a href="#RollbackConfig">
RollbackConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DEPRECATED.
The config this MachineDeployment is rolling back to. Will be cleared after rollback is done.</p>
</td>
</tr>
<tr>
<td>
<code>progressDeadlineSeconds</code>
</td>
<td>
<em>
*int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum time in seconds for a MachineDeployment to make progress before it
is considered to be failed. The MachineDeployment controller will continue to
process failed MachineDeployments and a condition with a ProgressDeadlineExceeded
reason will be surfaced in the MachineDeployment status. Note that progress will
not be estimated during the time a MachineDeployment is paused. This is not set
by default.</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="MachineDeploymentStatus">
<b>MachineDeploymentStatus</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineDeployment">MachineDeployment</a>)
</p>
<p>
<p>MachineDeploymentStatus is the most recently observed status of the MachineDeployment.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>observedGeneration</code>
</td>
<td>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>The generation observed by the MachineDeployment controller.</p>
</td>
</tr>
<tr>
<td>
<code>replicas</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Total number of non-terminated machines targeted by this MachineDeployment (their labels match the selector).</p>
</td>
</tr>
<tr>
<td>
<code>updatedReplicas</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Total number of non-terminated machines targeted by this MachineDeployment that have the desired template spec.</p>
</td>
</tr>
<tr>
<td>
<code>readyReplicas</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Total number of ready machines targeted by this MachineDeployment.</p>
</td>
</tr>
<tr>
<td>
<code>availableReplicas</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Total number of available machines (ready for at least minReadySeconds) targeted by this MachineDeployment.</p>
</td>
</tr>
<tr>
<td>
<code>unavailableReplicas</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Total number of unavailable machines targeted by this MachineDeployment. This is the total number of
machines that are still required for the MachineDeployment to have 100% available capacity. They may
either be machines that are running but not yet available or machines that still have not been created.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code>
</td>
<td>
<em>
<a href="#MachineDeploymentCondition">
[]MachineDeploymentCondition
</a>
</em>
</td>
<td>
<p>Represents the latest available observations of a MachineDeployment&rsquo;s current state.</p>
</td>
</tr>
<tr>
<td>
<code>collisionCount</code>
</td>
<td>
<em>
*int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Count of hash collisions for the MachineDeployment. The MachineDeployment controller uses this
field as a collision avoidance mechanism when it needs to create the name for the
newest MachineSet.</p>
</td>
</tr>
<tr>
<td>
<code>failedMachines</code>
</td>
<td>
<em>
<a href="#%2agithub.com%2fgardener%2fmachine-controller-manager%2fpkg%2fapis%2fmachine%2fv1alpha1.MachineSummary">
[]*github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1.MachineSummary
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>FailedMachines has summary of machines on which lastOperation Failed</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="MachineDeploymentStrategy">
<b>MachineDeploymentStrategy</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineDeploymentSpec">MachineDeploymentSpec</a>)
</p>
<p>
<p>MachineDeploymentStrategy describes how to replace existing machines with new ones.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code>
</td>
<td>
<em>
<a href="#MachineDeploymentStrategyType">
MachineDeploymentStrategyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Type of MachineDeployment. Can be &ldquo;Recreate&rdquo; or &ldquo;RollingUpdate&rdquo;. Default is RollingUpdate.</p>
</td>
</tr>
<tr>
<td>
<code>rollingUpdate</code>
</td>
<td>
<em>
<a href="#RollingUpdateMachineDeployment">
RollingUpdateMachineDeployment
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Rolling update config params. Present only if MachineDeploymentStrategyType =</p>
<h2>RollingUpdate.</h2>
<p>TODO: Update this to follow our convention for oneOf, whatever we decide it
to be.</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="MachineDeploymentStrategyType">
<b>MachineDeploymentStrategyType</b>
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#MachineDeploymentStrategy">MachineDeploymentStrategy</a>)
</p>
<p>
</p>
<br>
<h3 id="MachineOperationType">
<b>MachineOperationType</b>
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#LastOperation">LastOperation</a>)
</p>
<p>
<p>MachineOperationType is a label for the operation performed on a machine object.</p>
</p>
<br>
<h3 id="MachinePhase">
<b>MachinePhase</b>
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#CurrentStatus">CurrentStatus</a>)
</p>
<p>
<p>MachinePhase is a label for the condition of a machines at the current time.</p>
</p>
<br>
<h3 id="MachineSetCondition">
<b>MachineSetCondition</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineSetStatus">MachineSetStatus</a>)
</p>
<p>
<p>MachineSetCondition describes the state of a machine set at a certain point.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code>
</td>
<td>
<em>
<a href="#MachineSetConditionType">
MachineSetConditionType
</a>
</em>
</td>
<td>
<p>Type of machine set condition.</p>
</td>
</tr>
<tr>
<td>
<code>status</code>
</td>
<td>
<em>
<a href="#ConditionStatus">
ConditionStatus
</a>
</em>
</td>
<td>
<p>Status of the condition, one of True, False, Unknown.</p>
</td>
</tr>
<tr>
<td>
<code>lastTransitionTime</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The last time the condition transitioned from one status to another.</p>
</td>
</tr>
<tr>
<td>
<code>reason</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The reason for the condition&rsquo;s last transition.</p>
</td>
</tr>
<tr>
<td>
<code>message</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>A human readable message indicating details about the transition.</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="MachineSetConditionType">
<b>MachineSetConditionType</b>
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#MachineSetCondition">MachineSetCondition</a>)
</p>
<p>
<p>MachineSetConditionType is the condition on machineset object</p>
</p>
<br>
<h3 id="MachineSetSpec">
<b>MachineSetSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineSet">MachineSet</a>)
</p>
<p>
<p>MachineSetSpec is the specification of a MachineSet.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>replicas</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>selector</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>machineClass</code>
</td>
<td>
<em>
<a href="#ClassSpec">
ClassSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>template</code>
</td>
<td>
<em>
<a href="#MachineTemplateSpec">
MachineTemplateSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>minReadySeconds</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="MachineSetStatus">
<b>MachineSetStatus</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineSet">MachineSet</a>)
</p>
<p>
<p>MachineSetStatus holds the most recently observed status of MachineSet.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>replicas</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<p>Replicas is the number of actual replicas.</p>
</td>
</tr>
<tr>
<td>
<code>fullyLabeledReplicas</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>The number of pods that have labels matching the labels of the pod template of the replicaset.</p>
</td>
</tr>
<tr>
<td>
<code>readyReplicas</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>The number of ready replicas for this replica set.</p>
</td>
</tr>
<tr>
<td>
<code>availableReplicas</code>
</td>
<td>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>The number of available replicas (ready for at least minReadySeconds) for this replica set.</p>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code>
</td>
<td>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the most recent generation observed by the controller.</p>
</td>
</tr>
<tr>
<td>
<code>machineSetCondition</code>
</td>
<td>
<em>
<a href="#MachineSetCondition">
[]MachineSetCondition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the latest available observations of a replica set&rsquo;s current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code>
</td>
<td>
<em>
<a href="#LastOperation">
LastOperation
</a>
</em>
</td>
<td>
<p>LastOperation performed</p>
</td>
</tr>
<tr>
<td>
<code>failedMachines</code>
</td>
<td>
<em>
<a href="#%5b%5dgithub.com%2fgardener%2fmachine-controller-manager%2fpkg%2fapis%2fmachine%2fv1alpha1.MachineSummary">
[]github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1.MachineSummary
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>FailedMachines has summary of machines on which lastOperation Failed</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="MachineSpec">
<b>MachineSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#Machine">Machine</a>, 
<a href="#MachineTemplateSpec">MachineTemplateSpec</a>)
</p>
<p>
<p>MachineSpec is the specification of a Machine.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>class</code>
</td>
<td>
<em>
<a href="#ClassSpec">
ClassSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class contains the machineclass attributes of a machine</p>
</td>
</tr>
<tr>
<td>
<code>providerID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderID represents the provider&rsquo;s unique ID given to a machine</p>
</td>
</tr>
<tr>
<td>
<code>nodeTemplate</code>
</td>
<td>
<em>
<a href="#NodeTemplateSpec">
NodeTemplateSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeTemplateSpec describes the data a node should have when created from a template</p>
</td>
</tr>
<tr>
<td>
<code>MachineConfiguration</code>
</td>
<td>
<em>
<a href="#MachineConfiguration">
MachineConfiguration
</a>
</em>
</td>
<td>
<p>
(Members of <code>MachineConfiguration</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>Configuration for the machine-controller.</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="MachineState">
<b>MachineState</b>
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#LastOperation">LastOperation</a>)
</p>
<p>
<p>MachineState is a current state of the machine.</p>
</p>
<br>
<h3 id="MachineStatus">
<b>MachineStatus</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#Machine">Machine</a>)
</p>
<p>
<p>MachineStatus holds the most recently observed status of Machine.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>node</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>Node string</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23nodecondition-v1-core">
[]Kubernetes core/v1.NodeCondition
</a>
</em>
</td>
<td>
<p>Conditions of this machine, same as node</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code>
</td>
<td>
<em>
<a href="#LastOperation">
LastOperation
</a>
</em>
</td>
<td>
<p>Last operation refers to the status of the last operation performed</p>
</td>
</tr>
<tr>
<td>
<code>currentStatus</code>
</td>
<td>
<em>
<a href="#CurrentStatus">
CurrentStatus
</a>
</em>
</td>
<td>
<p>Current status of the machine object</p>
</td>
</tr>
<tr>
<td>
<code>lastKnownState</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastKnownState can store details of the last known state of the VM by the plugins.
It can be used by future operation calls to determine current infrastucture state</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="MachineSummary">
<b>MachineSummary</b>
</h3>
<p>
<p>MachineSummary store the summary of machine.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>Name of the machine object</p>
</td>
</tr>
<tr>
<td>
<code>providerID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>ProviderID represents the provider&rsquo;s unique ID given to a machine</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code>
</td>
<td>
<em>
<a href="#LastOperation">
LastOperation
</a>
</em>
</td>
<td>
<p>Last operation refers to the status of the last operation performed</p>
</td>
</tr>
<tr>
<td>
<code>ownerRef</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>OwnerRef</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="MachineTemplateSpec">
<b>MachineTemplateSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineDeploymentSpec">MachineDeploymentSpec</a>, 
<a href="#MachineSetSpec">MachineSetSpec</a>)
</p>
<p>
<p>MachineTemplateSpec describes the data a machine should have when created from a template</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Standard object&rsquo;s metadata.
More info: <a href="https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata">https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata</a></p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code>
</td>
<td>
<em>
<a href="#MachineSpec">
MachineSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the desired behavior of the machine.
More info: <a href="https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status">https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status</a></p>
<br/>
<br/>
<table>
<tr>
<td>
<code>class</code>
</td>
<td>
<em>
<a href="#ClassSpec">
ClassSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class contains the machineclass attributes of a machine</p>
</td>
</tr>
<tr>
<td>
<code>providerID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderID represents the provider&rsquo;s unique ID given to a machine</p>
</td>
</tr>
<tr>
<td>
<code>nodeTemplate</code>
</td>
<td>
<em>
<a href="#NodeTemplateSpec">
NodeTemplateSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeTemplateSpec describes the data a node should have when created from a template</p>
</td>
</tr>
<tr>
<td>
<code>MachineConfiguration</code>
</td>
<td>
<em>
<a href="#MachineConfiguration">
MachineConfiguration
</a>
</em>
</td>
<td>
<p>
(Members of <code>MachineConfiguration</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>Configuration for the machine-controller.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="NodeTemplate">
<b>NodeTemplate</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineClass">MachineClass</a>)
</p>
<p>
<p>NodeTemplate contains subfields to track all node resources and other node info required to scale nodegroup from zero</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>capacity</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23resourcelist-v1-core">
Kubernetes core/v1.ResourceList
</a>
</em>
</td>
<td>
<p>Capacity contains subfields to track all node resources required to scale nodegroup from zero</p>
</td>
</tr>
<tr>
<td>
<code>instanceType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>Instance type of the node belonging to nodeGroup</p>
</td>
</tr>
<tr>
<td>
<code>region</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>Region of the expected node belonging to nodeGroup</p>
</td>
</tr>
<tr>
<td>
<code>zone</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>Zone of the expected node belonging to nodeGroup</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="NodeTemplateSpec">
<b>NodeTemplateSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineSpec">MachineSpec</a>)
</p>
<p>
<p>NodeTemplateSpec describes the data a node should have when created from a template</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23nodespec-v1-core">
Kubernetes core/v1.NodeSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeSpec describes the attributes that a node is created with.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>podCIDR</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>PodCIDR represents the pod IP range assigned to the node.</p>
</td>
</tr>
<tr>
<td>
<code>podCIDRs</code>
</td>
<td>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>podCIDRs represents the IP ranges assigned to the node for usage by Pods on that node. If this
field is specified, the 0th entry must match the podCIDR field. It may contain at most 1 value for
each of IPv4 and IPv6.</p>
</td>
</tr>
<tr>
<td>
<code>providerID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ID of the node assigned by the cloud provider in the format: <ProviderName>://<ProviderSpecificNodeID></p>
</td>
</tr>
<tr>
<td>
<code>unschedulable</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Unschedulable controls node schedulability of new pods. By default, node is schedulable.
More info: <a href="https://kubernetes.io/docs/concepts/nodes/node/#manual-node-administration">https://kubernetes.io/docs/concepts/nodes/node/#manual-node-administration</a></p>
</td>
</tr>
<tr>
<td>
<code>taints</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23taint-v1-core">
[]Kubernetes core/v1.Taint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>If specified, the node&rsquo;s taints.</p>
</td>
</tr>
<tr>
<td>
<code>configSource</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23nodeconfigsource-v1-core">
Kubernetes core/v1.NodeConfigSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deprecated. If specified, the source of the node&rsquo;s configuration.
The DynamicKubeletConfig feature gate must be enabled for the Kubelet to use this field.
This field is deprecated as of 1.22: <a href="https://git.k8s.io/enhancements/keps/sig-node/281-dynamic-kubelet-configuration">https://git.k8s.io/enhancements/keps/sig-node/281-dynamic-kubelet-configuration</a></p>
</td>
</tr>
<tr>
<td>
<code>externalID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deprecated. Not all kubelets will set this field. Remove field after 1.13.
see: <a href="https://issues.k8s.io/61966">https://issues.k8s.io/61966</a></p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="OpenStackMachineClass">
<b>OpenStackMachineClass</b>
</h3>
<p>
<p>OpenStackMachineClass TODO</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code>
</td>
<td>
<em>
<a href="#OpenStackMachineClassSpec">
OpenStackMachineClassSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<br/>
<br/>
<table>
<tr>
<td>
<code>imageID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>imageName</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>region</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>availabilityZone</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>flavorName</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>keyName</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>securityGroups</code>
</td>
<td>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tags</code>
</td>
<td>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>networkID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>networks</code>
</td>
<td>
<em>
<a href="#OpenStackNetwork">
[]OpenStackNetwork
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>subnetID</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>credentialsSecretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>podNetworkCidr</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>rootDiskSize</code>
</td>
<td>
<em>
int
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>useConfigDrive</code>
</td>
<td>
<em>
*bool
</em>
</td>
<td>
<p>in GB</p>
</td>
</tr>
<tr>
<td>
<code>serverGroupID</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="OpenStackMachineClassSpec">
<b>OpenStackMachineClassSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#OpenStackMachineClass">OpenStackMachineClass</a>)
</p>
<p>
<p>OpenStackMachineClassSpec is the specification of a OpenStackMachineClass.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>imageID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>imageName</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>region</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>availabilityZone</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>flavorName</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>keyName</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>securityGroups</code>
</td>
<td>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tags</code>
</td>
<td>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>networkID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>networks</code>
</td>
<td>
<em>
<a href="#OpenStackNetwork">
[]OpenStackNetwork
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>subnetID</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>credentialsSecretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>podNetworkCidr</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>rootDiskSize</code>
</td>
<td>
<em>
int
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>useConfigDrive</code>
</td>
<td>
<em>
*bool
</em>
</td>
<td>
<p>in GB</p>
</td>
</tr>
<tr>
<td>
<code>serverGroupID</code>
</td>
<td>
<em>
*string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="OpenStackNetwork">
<b>OpenStackNetwork</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#OpenStackMachineClassSpec">OpenStackMachineClassSpec</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>id</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>name</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
<p>takes priority before name</p>
</td>
</tr>
<tr>
<td>
<code>podNetwork</code>
</td>
<td>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="PacketMachineClassSpec">
<b>PacketMachineClassSpec</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#PacketMachineClass">PacketMachineClass</a>)
</p>
<p>
<p>PacketMachineClassSpec is the specification of a PacketMachineClass.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>facility</code>
</td>
<td>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>machineType</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>billingCycle</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>OS</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>projectID</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tags</code>
</td>
<td>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>sshKeys</code>
</td>
<td>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>userdata</code>
</td>
<td>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>secretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>credentialsSecretRef</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fkubernetes.io%2fdocs%2freference%2fgenerated%2fkubernetes-api%2fv1.19%2f%23secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="RollbackConfig">
<b>RollbackConfig</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineDeploymentSpec">MachineDeploymentSpec</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>revision</code>
</td>
<td>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>The revision to rollback to. If set to 0, rollback to the last revision.</p>
</td>
</tr>
</tbody>
</table>
<br>
<h3 id="RollingUpdateMachineDeployment">
<b>RollingUpdateMachineDeployment</b>
</h3>
<p>
(<em>Appears on:</em>
<a href="#MachineDeploymentStrategy">MachineDeploymentStrategy</a>)
</p>
<p>
<p>Spec to control the desired behavior of rolling update.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Type</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>maxUnavailable</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fgodoc.org%2fk8s.io%2fapimachinery%2fpkg%2futil%2fintstr%23IntOrString">
k8s.io/apimachinery/pkg/util/intstr.IntOrString
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum number of machines that can be unavailable during the update.
Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).
Absolute number is calculated from percentage by rounding down.
This can not be 0 if MaxSurge is 0.
By default, a fixed value of 1 is used.
Example: when this is set to 30%, the old MC can be scaled down to 70% of desired machines
immediately when the rolling update starts. Once new machines are ready, old MC
can be scaled down further, followed by scaling up the new MC, ensuring
that the total number of machines available at all times during the update is at
least 70% of desired machines.</p>
</td>
</tr>
<tr>
<td>
<code>maxSurge</code>
</td>
<td>
<em>
<a href="#https%3a%2f%2fgodoc.org%2fk8s.io%2fapimachinery%2fpkg%2futil%2fintstr%23IntOrString">
k8s.io/apimachinery/pkg/util/intstr.IntOrString
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum number of machines that can be scheduled above the desired number of
machines.
Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).
This can not be 0 if MaxUnavailable is 0.
Absolute number is calculated from percentage by rounding up.
By default, a value of 1 is used.
Example: when this is set to 30%, the new MC can be scaled up immediately when
the rolling update starts, such that the total number of old and new machines do not exceed
130% of desired machines. Once old machines have been killed,
new MC can be scaled up further, ensuring that total number of machines running
at any time during the update is atmost 130% of desired machines.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
