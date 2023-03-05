## Specification
### ProviderSpec Schema
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
by default, which is treated as infinite deadline.</p>
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
by default, which is treated as infinite deadline.</p>
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
<a href="#%2a..%2f..%2fpkg%2fapis%2fmachine%2fv1alpha1.MachineSummary">
[]*../../pkg/apis/machine/v1alpha1.MachineSummary
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
<a href="#%5b%5d..%2f..%2fpkg%2fapis%2fmachine%2fv1alpha1.MachineSummary">
[]../../pkg/apis/machine/v1alpha1.MachineSummary
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
<p>Deprecated: Previously used to specify the source of the node&rsquo;s configuration for the DynamicKubeletConfig feature. This feature is removed.</p>
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
