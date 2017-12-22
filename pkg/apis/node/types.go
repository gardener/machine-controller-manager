/*
Copyright 2017 The Gardener Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package node

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Provider is the type of the provider to be used
type Provider string

// Valid providers
const (
	ProviderAWS Provider = "aws"
)

// WARNING!
// IF YOU MODIFY ANY OF THE TYPES HERE COPY THEM TO ../types.go
// AND RUN  make generate-files

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AWSInstanceClass TODO
type AWSInstanceClass struct {

	metav1.ObjectMeta 

	metav1.TypeMeta 

	Spec AWSInstanceClassSpec 
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AWSInstanceClassList is a collection of AWSInstanceClasses.
type AWSInstanceClassList struct {
	
	metav1.TypeMeta 

	metav1.ListMeta 

	Items []AWSInstanceClass 
}

// AWSInstanceClassSpec is the specification of a Shoot cluster.
type AWSInstanceClassSpec struct {
	AMI               string                      
	AvailabilityZone  string                      
	BlockDevices      []AWSBlockDeviceMappingSpec 
	EbsOptimized      bool                        
	IAM               AWSIAMProfileSpec           
	InstanceType      string                      
	KeyName           string                      
	Monitoring        bool                        
	NetworkInterfaces []AWSNetworkInterfaceSpec   
	Tags              map[string]string           
	SecretRef         *corev1.SecretReference
}

type AWSBlockDeviceMappingSpec struct {

	// The device name exposed to the instance (for example, /dev/sdh or xvdh).
	DeviceName string 

	// Parameters used to automatically set up EBS volumes when the instance is
	// launched.
	Ebs AWSEbsBlockDeviceSpec 

	// Suppresses the specified device included in the block device mapping of the
	// AMI.
	NoDevice string 

	// The virtual device name (ephemeralN). Instance store volumes are numbered
	// starting from 0. An instance type with 2 available instance store volumes
	// can specify mappings for ephemeral0 and ephemeral1.The number of available
	// instance store volumes depends on the instance type. After you connect to
	// the instance, you must mount the volume.
	//
	// Constraints: For M3 instances, you must specify instance store volumes in
	// the block device mapping for the instance. When you launch an M3 instance,
	// we ignore any instance store volumes specified in the block device mapping
	// for the AMI.
	VirtualName string 
}

// Describes a block device for an EBS volume.
// Please also see https://docs.aws.amazon.com/goto/WebAPI/ec2-2016-11-15/EbsBlockDevice
type AWSEbsBlockDeviceSpec struct {

	// Indicates whether the EBS volume is deleted on instance termination.
	DeleteOnTermination bool 

	// Indicates whether the EBS volume is encrypted. Encrypted Amazon EBS volumes
	// may only be attached to instances that support Amazon EBS encryption.
	Encrypted bool 

	// The number of I/O operations per second (IOPS) that the volume supports.
	// For io1, this represents the number of IOPS that are provisioned for the
	// volume. For gp2, this represents the baseline performance of the volume and
	// the rate at which the volume accumulates I/O credits for bursting. For more
	// information about General Purpose SSD baseline performance, I/O credits,
	// and bursting, see Amazon EBS Volume Types (http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html)
	// in the Amazon Elastic Compute Cloud User Guide.
	//
	// Constraint: Range is 100-20000 IOPS for io1 volumes and 100-10000 IOPS for
	// gp2 volumes.
	//
	// Condition: This parameter is required for requests to create io1 volumes;
	// it is not used in requests to create gp2, st1, sc1, or standard volumes.
	Iops int64 

	// The size of the volume, in GiB.
	//
	// Constraints: 1-16384 for General Purpose SSD (gp2), 4-16384 for Provisioned
	// IOPS SSD (io1), 500-16384 for Throughput Optimized HDD (st1), 500-16384 for
	// Cold HDD (sc1), and 1-1024 for Magnetic (standard) volumes. If you specify
	// a snapshot, the volume size must be equal to or larger than the snapshot
	// size.
	//
	// Default: If you're creating the volume from a snapshot and don't specify
	// a volume size, the default is the snapshot size.
	VolumeSize int64 

	// The volume type: gp2, io1, st1, sc1, or standard.
	//
	// Default: standard
	VolumeType string 
}

// Describes an IAM instance profile.
type AWSIAMProfileSpec struct {

	// The Amazon Resource Name (ARN) of the instance profile.
	ARN string 

	// The name of the instance profile.
	Name string 
}

// Describes a network interface.
// Please also see https://docs.aws.amazon.com/goto/WebAPI/ec2-2016-11-15/InstanceAWSNetworkInterfaceSpecification
type AWSNetworkInterfaceSpec struct {

	// Indicates whether to assign a public IPv4 address to an instance you launch
	// in a VPC. The public IP address can only be assigned to a network interface
	// for eth0, and can only be assigned to a new network interface, not an existing
	// one. You cannot specify more than one network interface in the request. If
	// launching into a default subnet, the default value is true.
	AssociatePublicIPAddress bool 

	// If set to true, the interface is deleted when the instance is terminated.
	// You can specify true only if creating a new network interface when launching
	// an instance.
	DeleteOnTermination bool 

	// The description of the network interface. Applies only if creating a network
	// interface when launching an instance.
	Description string 

	// The IDs of the security groups for the network interface. Applies only if
	// creating a network interface when launching an instance.
	SecurityGroupID []string 

	// The ID of the subnet associated with the network string. Applies only if
	// creating a network interface when launching an instance.
	SubnetID string 
}

// InstancePhase is a label for the condition of a instances at the current time.
type InstancePhase string

// These are the valid statuses of instances.
const (
	// InstancePending means that the instance is being created
	InstancePending InstancePhase = "Pending"
	// InstancePending means that instance is present on provider but hasn't joined cluster yet
	InstanceAvailable InstancePhase = "Available"
	// InstanceRunning means node is ready and running succesfully
	InstanceRunning InstancePhase = "Running"
	// InstanceRunning means node is terminating
	InstanceTerminating InstancePhase = "Terminating"
	// InstanceUnknown indicates that the node is not ready at the movement
	InstanceUnknown InstancePhase = "Unknown"
	// InstanceFailed means operation failed leading to instance status failure
	InstanceFailed InstancePhase  = "Failed"
)

// InstancePhase is a label for the condition of a instances at the current time.
type InstanceState string

// These are the valid statuses of instances.
const (
	// InstancePending means there are operations pending on this instance state
	InstanceStateProcessing InstanceState = "Processing"
	// InstanceFailed means operation failed leading to instance status failure
	InstanceStateFailed InstanceState  = "Failed"
	// InstanceUnknown indicates that the node is not ready at the movement
	InstanceStateSuccessful InstanceState = "Successful"
)

// The below types are used by kube_client and api_server.

type ConditionStatus string

// These are valid condition statuses. "ConditionTrue" means a resource is in the condition;
// "ConditionFalse" means a resource is not in the condition; "ConditionUnknown" means kubernetes
// can't decide if a resource is in the condition or not. In the future, we could add other
// intermediate conditions, e.g. ConditionDegraded.
const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

// +genclient
// +genclient:nonNamespaced
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Instance TODO
type Instance struct {

	metav1.ObjectMeta 

	metav1.TypeMeta 

	Spec InstanceSpec 

	Status InstanceStatus 
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InstanceList is a collection of Instances.
type InstanceList struct {
	metav1.TypeMeta 

	metav1.ListMeta 

	Items []Instance 
}

// InstanceSpec is the specification of a Shoot cluster.
type InstanceSpec struct {

	Class ClassSpec 

	ProviderID string 
}


// PodTemplateSpec describes the data a pod should have when created from a template
type InstanceTemplateSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta 

	// Specification of the desired behavior of the pod.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status
	Spec InstanceSpec 
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodTemplate describes a template for creating copies of a predefined pod.
type InstanceTemplate struct {
	metav1.TypeMeta 
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta 

	// Template defines the pods that will be created from this pod template.
	// https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status
	Template InstanceTemplateSpec 
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodTemplateList is a list of PodTemplates.
type InstanceTemplateList struct {
	metav1.TypeMeta 
	// Standard list metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds
	metav1.ListMeta 

	// List of pod templates
	Items []InstanceTemplate 
}


// ClassSpec is the class specification of instance
type ClassSpec struct {
	// API group to which it belongs
	APIGroup	string 					
    // Kind for instance class
    Kind		string 					
    // Name of instance class
    Name 		string 					
}

//type CurrentStatus 
type CurrentStatus struct {
	// API group to which it belongs
	Phase				InstancePhase 			
   	// Name of instance class
    TimeoutActive 		bool				
    // Last update time of current status
	LastUpdateTime  	metav1.Time 				
}

// InstanceStatus TODO
type InstanceStatus struct {
	// Node string
	Node 				string 					
	// Conditions of this instance, same as node
	Conditions          []corev1.NodeCondition 	
	// Last operation refers to the status of the last operation performed
	LastOperation		LastOperation 			
	// Current status of the instance object
	CurrentStatus		CurrentStatus 			
}

// LastOperation
type LastOperation struct {
	// Description of the current operation
	Description 		string  				
	// Last update time of current operation
	LastUpdateTime  	metav1.Time 			
	// State of operation
	State				InstanceState			
	// Type of operation
	Type				string 					
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InstanceSet TODO
type InstanceSet struct {

	metav1.ObjectMeta 

	metav1.TypeMeta 

	Spec InstanceSetSpec 

	Status InstanceSetStatus 

}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InstanceSetList is a collection of InstanceSet.
type InstanceSetList struct {
	metav1.TypeMeta 

	metav1.ListMeta 

	Items []InstanceSet 
}

// InstanceSetSpec is the specification of a Shoot cluster.
type InstanceSetSpec struct {

	Replicas 			int32 					

	Selector 			*metav1.LabelSelector 	

	InstanceClass 		ClassSpec 				

	Template 			InstanceTemplateSpec 	

	MinReadySeconds  	int 					
}

type InstanceSetConditionType string

// These are valid conditions of a replica set.
const (
	// ReplicaSetReplicaFailure is added in a replica set when one of its pods fails to be created
	// due to insufficient quota, limit ranges, pod security policy, node selectors, etc. or deleted
	// due to kubelet being down or finalizers are failing.
	InstanceSetReplicaFailure InstanceSetConditionType = "ReplicaFailure"
)

// ReplicaSetCondition describes the state of a replica set at a certain point.
type InstanceSetCondition struct {
	// Type of replica set condition.
	Type InstanceSetConditionType 
	// Status of the condition, one of True, False, Unknown.
	Status ConditionStatus 
	// The last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time 
	// The reason for the condition's last transition.
	Reason string 
	// A human readable message indicating details about the transition.
	Message string 
}

// InstanceSetStatus TODO
type InstanceSetStatus struct {
	// Conditions of this instance, same as node
	LastOperation			LastOperation 						

	Replicas 				int32  					

	FullyLabeledReplicas	int32					

	ReadyReplicas			int32  					

	AvailableReplicas 		int32    					

	Conditions 				[]InstanceSetCondition  

	ObservedGeneration 		int64 					
}


/***************** InstanceDeploymennt APIs. ******************/

// +genclient
// +genclient:nonNamespaced
// +genclient:method=GetScale,verb=get,subresource=scale,result=Scale
// +genclient:method=UpdateScale,verb=update,subresource=scale,input=Scale,result=Scale
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type InstanceDeployment struct {
	metav1.TypeMeta
	// +optional
	metav1.ObjectMeta

	// Specification of the desired behavior of the Deployment.
	// +optional
	Spec InstanceDeploymentSpec

	// Most recently observed status of the Deployment.
	// +optional
	Status InstanceDeploymentStatus
}

type InstanceDeploymentSpec struct {
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas int32

	// Label selector for pods. Existing ReplicaSets whose pods are
	// selected by this will be the ones affected by this deployment.
	// +optional
	Selector *metav1.LabelSelector

	// Template describes the pods that will be created.
	Template InstanceTemplateSpec

	// The deployment strategy to use to replace existing pods with new ones.
	// +optional
	Strategy InstanceDeploymentStrategy

	// Minimum number of seconds for which a newly created pod should be ready
	// without any of its container crashing, for it to be considered available.
	// Defaults to 0 (pod will be considered available as soon as it is ready)
	// +optional
	MinReadySeconds int32

	// The number of old ReplicaSets to retain to allow rollback.
	// This is a pointer to distinguish between explicit zero and not specified.
	// +optional
	RevisionHistoryLimit *int32

	// Indicates that the deployment is paused and will not be processed by the
	// deployment controller.
	// +optional
	Paused bool

	// DEPRECATED.
	// The config this deployment is rolling back to. Will be cleared after rollback is done.
	// +optional
	RollbackTo *RollbackConfig

	// The maximum time in seconds for a deployment to make progress before it
	// is considered to be failed. The deployment controller will continue to
	// process failed deployments and a condition with a ProgressDeadlineExceeded
	// reason will be surfaced in the deployment status. Note that progress will
	// not be estimated during the time a deployment is paused. This is not set
	// by default.
	// +optional
	ProgressDeadlineSeconds *int32
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DEPRECATED.
// DeploymentRollback stores the information required to rollback a deployment.
type InstanceDeploymentRollback struct {
	metav1.TypeMeta
	// Required: This must match the Name of a deployment.
	Name string
	// The annotations to be updated to a deployment
	// +optional
	UpdatedAnnotations map[string]string
	// The config of this deployment rollback.
	RollbackTo RollbackConfig
}

// DEPRECATED.
type RollbackConfig struct {
	// The revision to rollback to. If set to 0, rollback to the last revision.
	// +optional
	Revision int64
}

const (
	// DefaultDeploymentUniqueLabelKey is the default key of the selector that is added
	// to existing RCs (and label key that is added to its pods) to prevent the existing RCs
	// to select new pods (and old pods being select by new RC).
	DefaultInstanceDeploymentUniqueLabelKey string = "instance-template-hash"
)

type InstanceDeploymentStrategy struct {
	// Type of deployment. Can be "Recreate" or "RollingUpdate". Default is RollingUpdate.
	// +optional
	Type InstanceDeploymentStrategyType

	// Rolling update config params. Present only if DeploymentStrategyType =
	// RollingUpdate.
	//---
	// TODO: Update this to follow our convention for oneOf, whatever we decide it
	// to be.
	// +optional
	RollingUpdate *RollingUpdateInstanceDeployment
}

type InstanceDeploymentStrategyType string

const (
	// Kill all existing pods before creating new ones.
	RecreateInstanceDeploymentStrategyType InstanceDeploymentStrategyType = "Recreate"

	// Replace the old RCs by new one using rolling update i.e gradually scale down the old RCs and scale up the new one.
	RollingUpdateInstanceDeploymentStrategyType InstanceDeploymentStrategyType = "RollingUpdate"
)

// Spec to control the desired behavior of rolling update.
type RollingUpdateInstanceDeployment struct {
	// The maximum number of pods that can be unavailable during the update.
	// Value can be an absolute number (ex: 5) or a percentage of total pods at the start of update (ex: 10%).
	// Absolute number is calculated from percentage by rounding down.
	// This can not be 0 if MaxSurge is 0.
	// By default, a fixed value of 1 is used.
	// Example: when this is set to 30%, the old RC can be scaled down by 30%
	// immediately when the rolling update starts. Once new pods are ready, old RC
	// can be scaled down further, followed by scaling up the new RC, ensuring
	// that at least 70% of original number of pods are available at all times
	// during the update.
	// +optional
	MaxUnavailable *intstr.IntOrString

	// The maximum number of pods that can be scheduled above the original number of
	// pods.
	// Value can be an absolute number (ex: 5) or a percentage of total pods at
	// the start of the update (ex: 10%). This can not be 0 if MaxUnavailable is 0.
	// Absolute number is calculated from percentage by rounding up.
	// By default, a value of 1 is used.
	// Example: when this is set to 30%, the new RC can be scaled up by 30%
	// immediately when the rolling update starts. Once old pods have been killed,
	// new RC can be scaled up further, ensuring that total number of pods running
	// at any time during the update is atmost 130% of original pods.
	// +optional
	MaxSurge *intstr.IntOrString
}

type InstanceDeploymentStatus struct {
	// The generation observed by the deployment controller.
	// +optional
	ObservedGeneration int64

	// Total number of non-terminated pods targeted by this deployment (their labels match the selector).
	// +optional
	Replicas int32

	// Total number of non-terminated pods targeted by this deployment that have the desired template spec.
	// +optional
	UpdatedReplicas int32

	// Total number of ready pods targeted by this deployment.
	// +optional
	ReadyReplicas int32

	// Total number of available pods (ready for at least minReadySeconds) targeted by this deployment.
	// +optional
	AvailableReplicas int32

	// Total number of unavailable pods targeted by this deployment. This is the total number of
	// pods that are still required for the deployment to have 100% available capacity. They may
	// either be pods that are running but not yet available or pods that still have not been created.
	// +optional
	UnavailableReplicas int32

	// Represents the latest available observations of a deployment's current state.
	Conditions []InstanceDeploymentCondition

	// Count of hash collisions for the Deployment. The Deployment controller uses this
	// field as a collision avoidance mechanism when it needs to create the name for the
	// newest ReplicaSet.
	// +optional
	CollisionCount *int32
}

type InstanceDeploymentConditionType string

// These are valid conditions of a deployment.
const (
	// Available means the deployment is available, ie. at least the minimum available
	// replicas required are up and running for at least minReadySeconds.
	InstanceDeploymentAvailable InstanceDeploymentConditionType = "Available"
	// Progressing means the deployment is progressing. Progress for a deployment is
	// considered when a new replica set is created or adopted, and when new pods scale
	// up or old pods scale down. Progress is not estimated for paused deployments or
	// when progressDeadlineSeconds is not specified.
	InstanceDeploymentProgressing InstanceDeploymentConditionType = "Progressing"
	// ReplicaFailure is added in a deployment when one of its pods fails to be created
	// or deleted.
	InstanceDeploymentReplicaFailure InstanceDeploymentConditionType = "ReplicaFailure"
)

// DeploymentCondition describes the state of a deployment at a certain point.
type InstanceDeploymentCondition struct {
	// Type of deployment condition.
	Type InstanceDeploymentConditionType
	// Status of the condition, one of True, False, Unknown.
	Status ConditionStatus
	// The last time this condition was updated.
	LastUpdateTime metav1.Time
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time
	// The reason for the condition's last transition.
	Reason string
	// A human readable message indicating details about the transition.
	Message string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type InstanceDeploymentList struct {
	metav1.TypeMeta
	// +optional
	metav1.ListMeta

	// Items is the list of deployments.
	Items []InstanceDeployment
}

// describes the attributes of a scale subresource
type ScaleSpec struct {
	// desired number of instances for the scaled object.
	// +optional
	Replicas int32
}

// represents the current status of a scale subresource.
type ScaleStatus struct {
	// actual number of observed instances of the scaled object.
	Replicas int32

	// label query over pods that should match the replicas count.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	// +optional
	Selector *metav1.LabelSelector
	
	
	TargetSelector string
}

// +genclient
// +genclient:noVerbs
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// represents a scaling request for a resource.
type Scale struct {
	metav1.TypeMeta
	// Standard object metadata; More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata.
	// +optional
	metav1.ObjectMeta

	// defines the behavior of the scale. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status.
	// +optional
	Spec ScaleSpec

	// current status of the scale. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status. Read-only.
	// +optional
	Status ScaleStatus
}
