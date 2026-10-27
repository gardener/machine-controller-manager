<p>Packages:</p>
<ul>
<li>
<a href="#machine.sapcloud.io%2fv1alpha1">machine.sapcloud.io/v1alpha1</a>
</li>
</ul>

<h2 id="machine.sapcloud.io/v1alpha1">machine.sapcloud.io/v1alpha1</h2>
<p>

</p>
Resource Types:
<ul>
<li>
<a href="#machine">Machine</a>
</li>
<li>
<a href="#machineclass">MachineClass</a>
</li>
<li>
<a href="#machinedeployment">MachineDeployment</a>
</li>
<li>
<a href="#machineset">MachineSet</a>
</li>
<li>
<a href="#machinetemplatespec">MachineTemplateSpec</a>
</li>
<li>
<a href="#nodetemplatespec">NodeTemplateSpec</a>
</li>
</ul>

<h3 id="classspec">ClassSpec
</h3>


<p>
(<em>Appears on:</em><a href="#machinesetspec">MachineSetSpec</a>, <a href="#machinespec">MachineSpec</a>)
</p>

<p>
ClassSpec is the class specification of machine
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>apiGroup</code></br>
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
<code>kind</code></br>
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
<code>name</code></br>
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


<h3 id="conditionstatus">ConditionStatus
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#machinedeploymentcondition">MachineDeploymentCondition</a>, <a href="#machinesetcondition">MachineSetCondition</a>)
</p>

<p>
ConditionStatus are valid condition statuses
</p>


<h3 id="currentstatus">CurrentStatus
</h3>


<p>
(<em>Appears on:</em><a href="#machinestatus">MachineStatus</a>)
</p>

<p>
CurrentStatus contains information about the current status of Machine.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>phase</code></br>
<em>
<a href="#machinephase">MachinePhase</a>
</em>
</td>
<td>
<p></p>
</td>
</tr>
<tr>
<td>
<code>timeoutActive</code></br>
<em>
boolean
</em>
</td>
<td>
<p></p>
</td>
</tr>
<tr>
<td>
<code>lastUpdateTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#time-v1-meta">Time</a>
</em>
</td>
<td>
<p>Last update time of current status</p>
</td>
</tr>
<tr>
<td>
<code>preserveExpiryTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#time-v1-meta">Time</a>
</em>
</td>
<td>
<p>PreserveExpiryTime is the time at which MCM will stop preserving the machine</p>
</td>
</tr>

</tbody>
</table>


<h3 id="inplaceupdatemachinedeployment">InPlaceUpdateMachineDeployment
</h3>


<p>
(<em>Appears on:</em><a href="#machinedeploymentstrategy">MachineDeploymentStrategy</a>)
</p>

<p>
InPlaceUpdateMachineDeployment specifies the spec to control the desired behavior of inplace update.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>maxUnavailable</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#intorstring-intstr-util">IntOrString</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum number of machines that can be unavailable during the update.<br />Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).<br />Absolute number is calculated from percentage by rounding down.<br />This can not be 0 if MaxSurge is 0.<br />Example: when this is set to 30%, the old machine set can be scaled down to 70% of desired machines<br />immediately when the update starts. Once new machines are ready, old machine set<br />can be scaled down further, followed by scaling up the new machine set, ensuring<br />that the total number of machines available at all times during the update is at<br />least 70% of desired machines.</p>
</td>
</tr>
<tr>
<td>
<code>maxSurge</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#intorstring-intstr-util">IntOrString</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum number of machines that can be scheduled above the desired number of<br />machines.<br />Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).<br />This can not be 0 if MaxUnavailable is 0.<br />Absolute number is calculated from percentage by rounding up.<br />Example: when this is set to 30%, the new machine set can be scaled up immediately when<br />the update starts, such that the total number of old and new machines does not exceed<br />130% of desired machines. Once old machines have been killed,<br />new machine set can be scaled up further, ensuring that total number of machines running<br />at any time during the update is utmost 130% of desired machines.</p>
</td>
</tr>
<tr>
<td>
<code>orchestrationType</code></br>
<em>
<a href="#orchestrationtype">OrchestrationType</a>
</em>
</td>
<td>
<p>OrchestrationType specifies the orchestration type for the inplace update.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="lastoperation">LastOperation
</h3>


<p>
(<em>Appears on:</em><a href="#machinesetstatus">MachineSetStatus</a>, <a href="#machinestatus">MachineStatus</a>, <a href="#machinesummary">MachineSummary</a>)
</p>

<p>
LastOperation suggests the last operation performed on the object
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>description</code></br>
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
<code>errorCode</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ErrorCode of the current operation if any</p>
</td>
</tr>
<tr>
<td>
<code>lastUpdateTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#time-v1-meta">Time</a>
</em>
</td>
<td>
<p>Last update time of current operation</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="#machinestate">MachineState</a>
</em>
</td>
<td>
<p>State of operation</p>
</td>
</tr>
<tr>
<td>
<code>type</code></br>
<em>
<a href="#machineoperationtype">MachineOperationType</a>
</em>
</td>
<td>
<p>Type of operation</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machine">Machine
</h3>


<p>
Machine is the representation of a physical or virtual machine.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>machine.sapcloud.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>Machine</code></td>
</tr>

<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta">ObjectMeta</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the <code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#machinespec">MachineSpec</a>
</em>
</td>
<td>
<p>Spec contains the specification of the machine</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#machinestatus">MachineStatus</a>
</em>
</td>
<td>
<p>Status contains fields depicting the status</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineclass">MachineClass
</h3>


<p>
MachineClass can be used to templatize and re-use provider configuration
across multiple Machines / MachineSets / MachineDeployments.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>machine.sapcloud.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>MachineClass</code></td>
</tr>

<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta">ObjectMeta</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the <code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>nodeTemplate</code></br>
<em>
<a href="#nodetemplate">NodeTemplate</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeTemplate contains subfields to track all node resources and other node info required to scale nodegroup from zero</p>
</td>
</tr>
<tr>
<td>
<code>credentialsSecretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#secretreference-v1-core">SecretReference</a>
</em>
</td>
<td>
<p>CredentialsSecretRef can optionally store the credentials (in this case the SecretRef does not need to store them).<br />This might be useful if multiple machine classes with the same credentials but different user-datas are used.</p>
</td>
</tr>
<tr>
<td>
<code>providerSpec</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<p>Provider-specific configuration to use during node creation.</p>
</td>
</tr>
<tr>
<td>
<code>provider</code></br>
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
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#secretreference-v1-core">SecretReference</a>
</em>
</td>
<td>
<p>SecretRef stores the necessary secrets such as credentials or userdata.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineconfiguration">MachineConfiguration
</h3>


<p>
(<em>Appears on:</em><a href="#machinespec">MachineSpec</a>)
</p>

<p>
MachineConfiguration describes the configurations useful for the machine-controller.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>drainTimeout</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineDraintimeout is the timeout after which machine is forcefully deleted.</p>
</td>
</tr>
<tr>
<td>
<code>healthTimeout</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineHealthTimeout is the timeout after which machine is declared unhealhty/failed.</p>
</td>
</tr>
<tr>
<td>
<code>creationTimeout</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineCreationTimeout is the timeout after which machinie creation is declared failed.</p>
</td>
</tr>
<tr>
<td>
<code>inPlaceUpdateTimeout</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineInPlaceUpdateTimeout is the timeout after which in-place update is declared failed.</p>
</td>
</tr>
<tr>
<td>
<code>machinePreserveTimeout</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachinePreserveTimeout is the timeout after which the machine preservation is stopped</p>
</td>
</tr>
<tr>
<td>
<code>disableHealthTimeout</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisableHealthTimeout if set to true, health timeout will be ignored. Leading to machine never being declared failed.<br />This is intended to be used only for in-place updates.</p>
</td>
</tr>
<tr>
<td>
<code>maxEvictRetries</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxEvictRetries is the number of retries that will be attempted while draining the node.</p>
</td>
</tr>
<tr>
<td>
<code>nodeConditions</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeConditions are the set of conditions if set to true for MachineHealthTimeOut, machine will be declared failed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinedeployment">MachineDeployment
</h3>


<p>
MachineDeployment enables declarative updates for machines and MachineSets.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>machine.sapcloud.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>MachineDeployment</code></td>
</tr>

<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta">ObjectMeta</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the <code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#machinedeploymentspec">MachineDeploymentSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the desired behavior of the MachineDeployment.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#machinedeploymentstatus">MachineDeploymentStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Most recently observed status of the MachineDeployment.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinedeploymentcondition">MachineDeploymentCondition
</h3>


<p>
(<em>Appears on:</em><a href="#machinedeploymentstatus">MachineDeploymentStatus</a>)
</p>

<p>
MachineDeploymentCondition describes the state of a MachineDeployment at a certain point.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>type</code></br>
<em>
<a href="#machinedeploymentconditiontype">MachineDeploymentConditionType</a>
</em>
</td>
<td>
<p>Type of MachineDeployment condition.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#conditionstatus">ConditionStatus</a>
</em>
</td>
<td>
<p>Status of the condition, one of True, False, Unknown.</p>
</td>
</tr>
<tr>
<td>
<code>lastUpdateTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#time-v1-meta">Time</a>
</em>
</td>
<td>
<p>The last time this condition was updated.</p>
</td>
</tr>
<tr>
<td>
<code>lastTransitionTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#time-v1-meta">Time</a>
</em>
</td>
<td>
<p>Last time the condition transitioned from one status to another.</p>
</td>
</tr>
<tr>
<td>
<code>reason</code></br>
<em>
string
</em>
</td>
<td>
<p>The reason for the condition's last transition.</p>
</td>
</tr>
<tr>
<td>
<code>message</code></br>
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


<h3 id="machinedeploymentconditiontype">MachineDeploymentConditionType
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#machinedeploymentcondition">MachineDeploymentCondition</a>)
</p>

<p>
MachineDeploymentConditionType are valid conditions of MachineDeployments
</p>


<h3 id="machinedeploymentspec">MachineDeploymentSpec
</h3>


<p>
(<em>Appears on:</em><a href="#machinedeployment">MachineDeployment</a>)
</p>

<p>
MachineDeploymentSpec is the specification of the desired behavior of the MachineDeployment.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>replicas</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Number of desired machines. This is a pointer to distinguish between explicit<br />zero and not specified. Defaults to 0.</p>
</td>
</tr>
<tr>
<td>
<code>selector</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#labelselector-v1-meta">LabelSelector</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Label selector for machines. Existing MachineSets whose machines are<br />selected by this will be the ones affected by this MachineDeployment.</p>
</td>
</tr>
<tr>
<td>
<code>template</code></br>
<em>
<a href="#machinetemplatespec">MachineTemplateSpec</a>
</em>
</td>
<td>
<p>Template describes the machines that will be created.</p>
</td>
</tr>
<tr>
<td>
<code>strategy</code></br>
<em>
<a href="#machinedeploymentstrategy">MachineDeploymentStrategy</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The MachineDeployment strategy to use to replace existing machines with new ones.</p>
</td>
</tr>
<tr>
<td>
<code>minReadySeconds</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Minimum number of seconds for which a newly created machine should be ready<br />without any of its container crashing, for it to be considered available.<br />Defaults to 0 (machine will be considered available as soon as it is ready)</p>
</td>
</tr>
<tr>
<td>
<code>revisionHistoryLimit</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>The number of old MachineSets to retain to allow rollback.<br />This is a pointer to distinguish between explicit zero and not specified.</p>
</td>
</tr>
<tr>
<td>
<code>paused</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates that the MachineDeployment is paused and will not be processed by the<br />MachineDeployment controller.</p>
</td>
</tr>
<tr>
<td>
<code>rollbackTo</code></br>
<em>
<a href="#rollbackconfig">RollbackConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DEPRECATED.<br />The config this MachineDeployment is rolling back to. Will be cleared after rollback is done.</p>
</td>
</tr>
<tr>
<td>
<code>progressDeadlineSeconds</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum time in seconds for a MachineDeployment to make progress before it<br />is considered to be failed. The MachineDeployment controller will continue to<br />process failed MachineDeployments and a condition with a ProgressDeadlineExceeded<br />reason will be surfaced in the MachineDeployment status. Note that progress will<br />not be estimated during the time a MachineDeployment is paused. This is not set<br />by default, which is treated as infinite deadline.</p>
</td>
</tr>
<tr>
<td>
<code>autoPreserveFailedMachineMax</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum number of failed machines in the machine deployment that can be auto-preserved.<br />In the gardener context, this number is derived from the AutoPreserveFailedMachineMax set at the worker level, distributed amongst the worker's machine deployments</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinedeploymentstatus">MachineDeploymentStatus
</h3>


<p>
(<em>Appears on:</em><a href="#machinedeployment">MachineDeployment</a>)
</p>

<p>
MachineDeploymentStatus is the most recently observed status of the MachineDeployment.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>observedGeneration</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>The generation observed by the MachineDeployment controller.</p>
</td>
</tr>
<tr>
<td>
<code>replicas</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Total number of non-terminated machines targeted by this MachineDeployment (their labels match the selector).</p>
</td>
</tr>
<tr>
<td>
<code>updatedReplicas</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Total number of non-terminated machines targeted by this MachineDeployment that have the desired template spec.</p>
</td>
</tr>
<tr>
<td>
<code>readyReplicas</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Total number of ready machines targeted by this MachineDeployment.</p>
</td>
</tr>
<tr>
<td>
<code>availableReplicas</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Total number of available machines (ready for at least minReadySeconds) targeted by this MachineDeployment.</p>
</td>
</tr>
<tr>
<td>
<code>preservedFailedReplicas</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>PreservedFailedReplicas is the number of preserved machines in Failed phase targeted by this MachineDeployment</p>
</td>
</tr>
<tr>
<td>
<code>unavailableReplicas</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Total number of unavailable machines targeted by this MachineDeployment. This is the total number of<br />machines that are still required for the MachineDeployment to have 100% available capacity. They may<br />either be machines that are running but not yet available, machines that still have not been created, or<br />machines that are preserved in Failed phase.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code></br>
<em>
<a href="#machinedeploymentcondition">MachineDeploymentCondition</a> array
</em>
</td>
<td>
<p>Represents the latest available observations of a MachineDeployment's current state.</p>
</td>
</tr>
<tr>
<td>
<code>collisionCount</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Count of hash collisions for the MachineDeployment. The MachineDeployment controller uses this<br />field as a collision avoidance mechanism when it needs to create the name for the<br />newest MachineSet.</p>
</td>
</tr>
<tr>
<td>
<code>failedMachines</code></br>
<em>
<a href="#machinesummary">MachineSummary</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>FailedMachines has summary of machines on which lastOperation Failed</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinedeploymentstrategy">MachineDeploymentStrategy
</h3>


<p>
(<em>Appears on:</em><a href="#machinedeploymentspec">MachineDeploymentSpec</a>)
</p>

<p>
MachineDeploymentStrategy describes how to replace existing machines with new ones.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>type</code></br>
<em>
<a href="#machinedeploymentstrategytype">MachineDeploymentStrategyType</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Type of MachineDeployment. Can be "Recreate" or "RollingUpdate". Default is RollingUpdate.</p>
</td>
</tr>
<tr>
<td>
<code>rollingUpdate</code></br>
<em>
<a href="#rollingupdatemachinedeployment">RollingUpdateMachineDeployment</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Rolling update config params. Present only if MachineDeploymentStrategyType =<br />RollingUpdate.</p>
</td>
</tr>
<tr>
<td>
<code>inPlaceUpdate</code></br>
<em>
<a href="#inplaceupdatemachinedeployment">InPlaceUpdateMachineDeployment</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>InPlaceUpdate update config params. Present only if MachineDeploymentStrategyType =<br />InPlaceUpdate.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinedeploymentstrategytype">MachineDeploymentStrategyType
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#machinedeploymentstrategy">MachineDeploymentStrategy</a>)
</p>

<p>
MachineDeploymentStrategyType are valid strategy types for rolling MachineDeployments
</p>


<h3 id="machineoperationtype">MachineOperationType
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#lastoperation">LastOperation</a>)
</p>

<p>
MachineOperationType is a label for the operation performed on a machine object.
</p>


<h3 id="machinephase">MachinePhase
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#currentstatus">CurrentStatus</a>)
</p>

<p>
MachinePhase is a label for the condition of a machine at the current time.
</p>


<h3 id="machineset">MachineSet
</h3>


<p>
MachineSet TODO
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>machine.sapcloud.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>MachineSet</code></td>
</tr>

<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta">ObjectMeta</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the <code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#machinesetspec">MachineSetSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#machinesetstatus">MachineSetStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinesetcondition">MachineSetCondition
</h3>


<p>
(<em>Appears on:</em><a href="#machinesetstatus">MachineSetStatus</a>)
</p>

<p>
MachineSetCondition describes the state of a machine set at a certain point.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>type</code></br>
<em>
<a href="#machinesetconditiontype">MachineSetConditionType</a>
</em>
</td>
<td>
<p>Type of machine set condition.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#conditionstatus">ConditionStatus</a>
</em>
</td>
<td>
<p>Status of the condition, one of True, False, Unknown.</p>
</td>
</tr>
<tr>
<td>
<code>lastTransitionTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The last time the condition transitioned from one status to another.</p>
</td>
</tr>
<tr>
<td>
<code>reason</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The reason for the condition's last transition.</p>
</td>
</tr>
<tr>
<td>
<code>message</code></br>
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


<h3 id="machinesetconditiontype">MachineSetConditionType
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#machinesetcondition">MachineSetCondition</a>)
</p>

<p>
MachineSetConditionType is the condition on machineset object
</p>


<h3 id="machinesetspec">MachineSetSpec
</h3>


<p>
(<em>Appears on:</em><a href="#machineset">MachineSet</a>)
</p>

<p>
MachineSetSpec is the specification of a MachineSet.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>replicas</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>
<tr>
<td>
<code>selector</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#labelselector-v1-meta">LabelSelector</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>
<tr>
<td>
<code>machineClass</code></br>
<em>
<a href="#classspec">ClassSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>
<tr>
<td>
<code>template</code></br>
<em>
<a href="#machinetemplatespec">MachineTemplateSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>
<tr>
<td>
<code>minReadySeconds</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>
<tr>
<td>
<code>autoPreserveFailedMachineMax</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinesetstatus">MachineSetStatus
</h3>


<p>
(<em>Appears on:</em><a href="#machineset">MachineSet</a>)
</p>

<p>
MachineSetStatus holds the most recently observed status of MachineSet.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>replicas</code></br>
<em>
integer
</em>
</td>
<td>
<p>Replicas is the number of actual replicas.</p>
</td>
</tr>
<tr>
<td>
<code>fullyLabeledReplicas</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>The number of pods that have labels matching the labels of the pod template of the replicaset.</p>
</td>
</tr>
<tr>
<td>
<code>readyReplicas</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>The number of ready replicas for this replica set.</p>
</td>
</tr>
<tr>
<td>
<code>availableReplicas</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>The number of available replicas (ready for at least minReadySeconds) for this replica set.</p>
</td>
</tr>
<tr>
<td>
<code>preservedFailedReplicas</code></br>
<em>
integer
</em>
</td>
<td>
<p>PreservedFailedReplicas is the number of preserved replicas in Failed phase for this replica set</p>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the most recent generation observed by the controller.</p>
</td>
</tr>
<tr>
<td>
<code>machineSetCondition</code></br>
<em>
<a href="#machinesetcondition">MachineSetCondition</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the latest available observations of a replica set's current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<p>LastOperation performed</p>
</td>
</tr>
<tr>
<td>
<code>failedMachines</code></br>
<em>
<a href="#machinesummary">MachineSummary</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>FailedMachines has summary of machines on which lastOperation Failed</p>
</td>
</tr>
<tr>
<td>
<code>autoPreserveFailedMachineCount</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>AutoPreserveFailedMachineCount has a count of the number of failed machines in the machineset that are currently auto-preserved</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinespec">MachineSpec
</h3>


<p>
(<em>Appears on:</em><a href="#machine">Machine</a>, <a href="#machinetemplatespec">MachineTemplateSpec</a>)
</p>

<p>
MachineSpec is the specification of a Machine.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>class</code></br>
<em>
<a href="#classspec">ClassSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class contains the machineclass attributes of a machine</p>
</td>
</tr>
<tr>
<td>
<code>providerID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderID represents the provider's unique ID given to a machine</p>
</td>
</tr>
<tr>
<td>
<code>nodeTemplate</code></br>
<em>
<a href="#nodetemplatespec">NodeTemplateSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeTemplateSpec describes the data a node should have when created from a template</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinestate">MachineState
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#lastoperation">LastOperation</a>)
</p>

<p>
MachineState is a current state of the operation.
</p>


<h3 id="machinestatus">MachineStatus
</h3>


<p>
(<em>Appears on:</em><a href="#machine">Machine</a>)
</p>

<p>
MachineStatus holds the most recently observed status of Machine.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>addresses</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#nodeaddress-v1-core">NodeAddress</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Addresses of this machines. This field is only present if the MCM provider runs without a target cluster and may<br />be used by clients to determine how to connect to the machine, instead of the `Node.status.addresses` field.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#nodecondition-v1-core">NodeCondition</a> array
</em>
</td>
<td>
<p>Conditions of this machine, same as node</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<p>Last operation refers to the status of the last operation performed</p>
</td>
</tr>
<tr>
<td>
<code>currentStatus</code></br>
<em>
<a href="#currentstatus">CurrentStatus</a>
</em>
</td>
<td>
<p>Current status of the machine object</p>
</td>
</tr>
<tr>
<td>
<code>lastKnownState</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastKnownState can store details of the last known state of the VM by the plugins.<br />It can be used by future operation calls to determine current infrastucture state</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinesummary">MachineSummary
</h3>


<p>
(<em>Appears on:</em><a href="#machinedeploymentstatus">MachineDeploymentStatus</a>, <a href="#machinesetstatus">MachineSetStatus</a>)
</p>

<p>
MachineSummary store the summary of machine.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>name</code></br>
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
<code>providerID</code></br>
<em>
string
</em>
</td>
<td>
<p>ProviderID represents the provider's unique ID given to a machine</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<p>Last operation refers to the status of the last operation performed</p>
</td>
</tr>
<tr>
<td>
<code>ownerRef</code></br>
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


<h3 id="machinetemplatespec">MachineTemplateSpec
</h3>


<p>
(<em>Appears on:</em><a href="#machinedeploymentspec">MachineDeploymentSpec</a>, <a href="#machinesetspec">MachineSetSpec</a>)
</p>

<p>
MachineTemplateSpec describes the data a machine should have when created from a template
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta">ObjectMeta</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the <code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#machinespec">MachineSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the desired behavior of the machine.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status</p>
</td>
</tr>

</tbody>
</table>


<h3 id="nodetemplate">NodeTemplate
</h3>


<p>
(<em>Appears on:</em><a href="#machineclass">MachineClass</a>)
</p>

<p>
NodeTemplate contains subfields to track all node resources and other node info required to scale nodegroup from zero
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>capacity</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#resourcelist-v1-core">ResourceList</a>
</em>
</td>
<td>
<p>Capacity contains subfields to track all node resources required to scale nodegroup from zero</p>
</td>
</tr>
<tr>
<td>
<code>virtualCapacity</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#resourcelist-v1-core">ResourceList</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VirtualCapacity represents the expected Node 'virtual' capacity ie comprising virtual extended resources.</p>
</td>
</tr>
<tr>
<td>
<code>instanceType</code></br>
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
<code>region</code></br>
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
<code>zone</code></br>
<em>
string
</em>
</td>
<td>
<p>Zone of the expected node belonging to nodeGroup</p>
</td>
</tr>
<tr>
<td>
<code>architecture</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CPU Architecture of the node belonging to nodeGroup</p>
</td>
</tr>

</tbody>
</table>


<h3 id="nodetemplatespec">NodeTemplateSpec
</h3>


<p>
(<em>Appears on:</em><a href="#machinespec">MachineSpec</a>)
</p>

<p>
NodeTemplateSpec describes the data a node should have when created from a template
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta">ObjectMeta</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the <code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#nodespec-v1-core">NodeSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeSpec describes the attributes that a node is created with.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="orchestrationtype">OrchestrationType
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#inplaceupdatemachinedeployment">InPlaceUpdateMachineDeployment</a>)
</p>

<p>
OrchestrationType specifies the orchestration type for the inplace update.
</p>


<h3 id="rollbackconfig">RollbackConfig
</h3>


<p>
(<em>Appears on:</em><a href="#machinedeploymentspec">MachineDeploymentSpec</a>)
</p>

<p>
RollbackConfig is the config to rollback a MachineDeployment
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>revision</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>The revision to rollback to. If set to 0, rollback to the last revision.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="rollingupdatemachinedeployment">RollingUpdateMachineDeployment
</h3>


<p>
(<em>Appears on:</em><a href="#machinedeploymentstrategy">MachineDeploymentStrategy</a>)
</p>

<p>
RollingUpdateMachineDeployment is the spec to control the desired behavior of rolling update.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>maxUnavailable</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#intorstring-intstr-util">IntOrString</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum number of machines that can be unavailable during the update.<br />Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).<br />Absolute number is calculated from percentage by rounding down.<br />This can not be 0 if MaxSurge is 0.<br />Example: when this is set to 30%, the old machine set can be scaled down to 70% of desired machines<br />immediately when the update starts. Once new machines are ready, old machine set<br />can be scaled down further, followed by scaling up the new machine set, ensuring<br />that the total number of machines available at all times during the update is at<br />least 70% of desired machines.</p>
</td>
</tr>
<tr>
<td>
<code>maxSurge</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#intorstring-intstr-util">IntOrString</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum number of machines that can be scheduled above the desired number of<br />machines.<br />Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).<br />This can not be 0 if MaxUnavailable is 0.<br />Absolute number is calculated from percentage by rounding up.<br />Example: when this is set to 30%, the new machine set can be scaled up immediately when<br />the update starts, such that the total number of old and new machines does not exceed<br />130% of desired machines. Once old machines have been killed,<br />new machine set can be scaled up further, ensuring that total number of machines running<br />at any time during the update is utmost 130% of desired machines.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="updateconfiguration">UpdateConfiguration
</h3>


<p>
(<em>Appears on:</em><a href="#inplaceupdatemachinedeployment">InPlaceUpdateMachineDeployment</a>, <a href="#rollingupdatemachinedeployment">RollingUpdateMachineDeployment</a>)
</p>

<p>
UpdateConfiguration specifies the udpate configuration for the deployment strategy.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>maxUnavailable</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#intorstring-intstr-util">IntOrString</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum number of machines that can be unavailable during the update.<br />Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).<br />Absolute number is calculated from percentage by rounding down.<br />This can not be 0 if MaxSurge is 0.<br />Example: when this is set to 30%, the old machine set can be scaled down to 70% of desired machines<br />immediately when the update starts. Once new machines are ready, old machine set<br />can be scaled down further, followed by scaling up the new machine set, ensuring<br />that the total number of machines available at all times during the update is at<br />least 70% of desired machines.</p>
</td>
</tr>
<tr>
<td>
<code>maxSurge</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#intorstring-intstr-util">IntOrString</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum number of machines that can be scheduled above the desired number of<br />machines.<br />Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).<br />This can not be 0 if MaxUnavailable is 0.<br />Absolute number is calculated from percentage by rounding up.<br />Example: when this is set to 30%, the new machine set can be scaled up immediately when<br />the update starts, such that the total number of old and new machines does not exceed<br />130% of desired machines. Once old machines have been killed,<br />new machine set can be scaled up further, ensuring that total number of machines running<br />at any time during the update is utmost 130% of desired machines.</p>
</td>
</tr>

</tbody>
</table>


