// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Seed represents an installation request for an external controller.
type Seed struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Spec contains the specification of this installation.
	Spec SeedSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// Status contains the status of this installation.
	Status SeedStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SeedList is a collection of Seeds.
type SeedList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of Seeds.
	Items []Seed `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// SeedTemplate is a template for creating a Seed object.
type SeedTemplate struct {
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Specification of the desired behavior of the Seed.
	// +optional
	Spec SeedSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// SeedSpec is the specification of a Seed.
type SeedSpec struct {
	// Backup holds the object store configuration for the backups of shoot (currently only etcd).
	// If it is not specified, then there won't be any backups taken for shoots associated with this seed.
	// If backup field is present in seed, then backups of the etcd from shoot control plane will be stored
	// under the configured object store.
	// +optional
	Backup *SeedBackup `json:"backup,omitempty" protobuf:"bytes,1,opt,name=backup"`
	// DNS contains DNS-relevant information about this seed cluster.
	DNS SeedDNS `json:"dns" protobuf:"bytes,2,opt,name=dns"`
	// Networks defines the pod, service and worker network of the Seed cluster.
	Networks SeedNetworks `json:"networks" protobuf:"bytes,3,opt,name=networks"`
	// Provider defines the provider type and region for this Seed cluster.
	Provider SeedProvider `json:"provider" protobuf:"bytes,4,opt,name=provider"`
	// SecretRef is a reference to a Secret object containing the Kubeconfig of the Kubernetes
	// cluster to be registered as Seed.
	// +optional
	SecretRef *corev1.SecretReference `json:"secretRef,omitempty" protobuf:"bytes,5,opt,name=secretRef"`
	// Taints describes taints on the seed.
	// +optional
	Taints []SeedTaint `json:"taints,omitempty" protobuf:"bytes,6,rep,name=taints"`
	// Volume contains settings for persistentvolumes created in the seed cluster.
	// +optional
	Volume *SeedVolume `json:"volume,omitempty" protobuf:"bytes,7,opt,name=volume"`
	// Settings contains certain settings for this seed cluster.
	// +optional
	Settings *SeedSettings `json:"settings,omitempty" protobuf:"bytes,8,opt,name=settings"`
	// Ingress configures Ingress specific settings of the Seed cluster. This field is immutable.
	// +optional
	Ingress *Ingress `json:"ingress,omitempty" protobuf:"bytes,9,opt,name=ingress"`
}

// SeedStatus is the status of a Seed.
type SeedStatus struct {
	// Gardener holds information about the Gardener which last acted on the Shoot.
	// +optional
	Gardener *Gardener `json:"gardener,omitempty" protobuf:"bytes,1,opt,name=gardener"`
	// KubernetesVersion is the Kubernetes version of the seed cluster.
	// +optional
	KubernetesVersion *string `json:"kubernetesVersion,omitempty" protobuf:"bytes,2,opt,name=kubernetesVersion"`
	// Conditions represents the latest available observations of a Seed's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,3,rep,name=conditions"`
	// ObservedGeneration is the most recent generation observed for this Seed. It corresponds to the
	// Seed's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,4,opt,name=observedGeneration"`
	// ClusterIdentity is the identity of the Seed cluster. This field is immutable.
	// +optional
	ClusterIdentity *string `json:"clusterIdentity,omitempty" protobuf:"bytes,5,opt,name=clusterIdentity"`
	// Capacity represents the total resources of a seed.
	// +optional
	Capacity corev1.ResourceList `json:"capacity,omitempty" protobuf:"bytes,6,rep,name=capacity"`
	// Allocatable represents the resources of a seed that are available for scheduling.
	// Defaults to Capacity.
	// +optional
	Allocatable corev1.ResourceList `json:"allocatable,omitempty" protobuf:"bytes,7,rep,name=allocatable"`
	// ClientCertificateExpirationTimestamp is the timestamp at which gardenlet's client certificate expires.
	// +optional
	ClientCertificateExpirationTimestamp *metav1.Time `json:"clientCertificateExpirationTimestamp,omitempty" protobuf:"bytes,8,opt,name=clientCertificateExpirationTimestamp"`
}

// SeedBackup contains the object store configuration for backups for shoot (currently only etcd).
type SeedBackup struct {
	// Provider is a provider name. This field is immutable.
	Provider string `json:"provider" protobuf:"bytes,1,opt,name=provider"`
	// ProviderConfig is the configuration passed to BackupBucket resource.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,2,opt,name=providerConfig"`
	// Region is a region name. This field is immutable.
	// +optional
	Region *string `json:"region,omitempty" protobuf:"bytes,3,opt,name=region"`
	// SecretRef is a reference to a Secret object containing the cloud provider credentials for
	// the object store where backups should be stored. It should have enough privileges to manipulate
	// the objects as well as buckets.
	SecretRef corev1.SecretReference `json:"secretRef" protobuf:"bytes,4,opt,name=secretRef"`
}

// SeedDNS contains DNS-relevant information about this seed cluster.
type SeedDNS struct {
	// IngressDomain is the domain of the Seed cluster pointing to the ingress controller endpoint. It will be used
	// to construct ingress URLs for system applications running in Shoot clusters. This field is immutable.
	// Deprecated: This field is deprecated and will be removed in a future version of Gardener. Use spec.ingress.domain instead.
	// +optional
	IngressDomain *string `json:"ingressDomain,omitempty" protobuf:"bytes,1,opt,name=ingressDomain"`
	// Provider configures a DNSProvider
	// +optional
	Provider *SeedDNSProvider `json:"provider,omitempty" protobuf:"bytes,2,opt,name=provider"`
}

// SeedDNSProvider configures a DNSProvider for Seeds
type SeedDNSProvider struct {
	// Type describes the type of the dns-provider, for example `aws-route53`
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
	// SecretRef is a reference to a Secret object containing cloud provider credentials used for registering external domains.
	SecretRef corev1.SecretReference `json:"secretRef" protobuf:"bytes,2,opt,name=secretRef"`
	// Domains contains information about which domains shall be included/excluded for this provider.
	// Deprecated: This field is deprecated and will be removed in a future version of Gardener.
	// +optional
	Domains *DNSIncludeExclude `json:"domains,omitempty" protobuf:"bytes,3,opt,name=domains"`
	// Zones contains information about which hosted zones shall be included/excluded for this provider.
	// Deprecated: This field is deprecated and will be removed in a future version of Gardener.
	// +optional
	Zones *DNSIncludeExclude `json:"zones,omitempty" protobuf:"bytes,4,opt,name=zones"`
}

// Ingress configures the Ingress specific settings of the Seed cluster
type Ingress struct {
	// Domain specifies the IngressDomain of the Seed cluster pointing to the ingress controller endpoint. It will be used
	// to construct ingress URLs for system applications running in Shoot clusters. Once set this field is immutable.
	Domain string `json:"domain" protobuf:"bytes,1,opt,name=domain"`
	// Controller configures a Gardener managed Ingress Controller listening on the ingressDomain
	Controller IngressController `json:"controller" protobuf:"bytes,2,opt,name=controller"`
}

// IngressController enables a Gardener managed Ingress Controller listening on the ingressDomain
type IngressController struct {
	// Kind defines which kind of IngressController to use, for example `nginx`
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	// ProviderConfig specifies infrastructure specific configuration for the ingressController
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,2,opt,name=providerConfig"`
}

// SeedNetworks contains CIDRs for the pod, service and node networks of a Kubernetes cluster.
type SeedNetworks struct {
	// Nodes is the CIDR of the node network. This field is immutable.
	// +optional
	Nodes *string `json:"nodes,omitempty" protobuf:"bytes,1,opt,name=nodes"`
	// Pods is the CIDR of the pod network. This field is immutable.
	Pods string `json:"pods" protobuf:"bytes,2,opt,name=pods"`
	// Services is the CIDR of the service network. This field is immutable.
	Services string `json:"services" protobuf:"bytes,3,opt,name=services"`
	// ShootDefaults contains the default networks CIDRs for shoots.
	// +optional
	ShootDefaults *ShootNetworks `json:"shootDefaults,omitempty" protobuf:"bytes,4,opt,name=shootDefaults"`
	// BlockCIDRs is a list of network addresses that should be blocked for shoot control plane components running
	// in the seed cluster.
	// +optional
	BlockCIDRs []string `json:"blockCIDRs,omitempty" protobuf:"bytes,5,rep,name=blockCIDRs"`
	// IPFamilies specifies the IP protocol versions to use for seed networking. This field is immutable.
	// See https://github.com/gardener/gardener/blob/master/docs/usage/ipv6.md.
	// Defaults to ["IPv4"].
	// +optional
	IPFamilies []IPFamily `json:"ipFamilies,omitempty" protobuf:"bytes,6,rep,name=ipFamilies,casttype=IPFamily"`
}

// ShootNetworks contains the default networks CIDRs for shoots.
type ShootNetworks struct {
	// Pods is the CIDR of the pod network.
	// +optional
	Pods *string `json:"pods,omitempty" protobuf:"bytes,1,opt,name=pods"`
	// Services is the CIDR of the service network.
	// +optional
	Services *string `json:"services,omitempty" protobuf:"bytes,2,opt,name=services"`
}

// SeedProvider defines the provider-specific information of this Seed cluster.
type SeedProvider struct {
	// Type is the name of the provider.
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
	// ProviderConfig is the configuration passed to Seed resource.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,2,opt,name=providerConfig"`
	// Region is a name of a region.
	Region string `json:"region" protobuf:"bytes,3,opt,name=region"`
	// Zones is the list of availability zones the seed cluster is deployed to.
	// +optional
	Zones []string `json:"zones,omitempty" protobuf:"bytes,4,rep,name=zones"`
}

// SeedSettings contains certain settings for this seed cluster.
type SeedSettings struct {
	// ExcessCapacityReservation controls the excess capacity reservation for shoot control planes in the seed.
	// +optional
	ExcessCapacityReservation *SeedSettingExcessCapacityReservation `json:"excessCapacityReservation,omitempty" protobuf:"bytes,1,opt,name=excessCapacityReservation"`
	// Scheduling controls settings for scheduling decisions for the seed.
	// +optional
	Scheduling *SeedSettingScheduling `json:"scheduling,omitempty" protobuf:"bytes,2,opt,name=scheduling"`

	// ShootDNS is tombstoned to show why 3 is reserved protobuf tag.
	// ShootDNS *SeedSettingShootDNS `json:"shootDNS,omitempty" protobuf:"bytes,3,opt,name=shootDNS"`

	// LoadBalancerServices controls certain settings for services of type load balancer that are created in the seed.
	// +optional
	LoadBalancerServices *SeedSettingLoadBalancerServices `json:"loadBalancerServices,omitempty" protobuf:"bytes,4,opt,name=loadBalancerServices"`
	// VerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the seed.
	// +optional
	VerticalPodAutoscaler *SeedSettingVerticalPodAutoscaler `json:"verticalPodAutoscaler,omitempty" protobuf:"bytes,5,opt,name=verticalPodAutoscaler"`
	// SeedSettingOwnerChecks controls certain owner checks settings for shoots scheduled on this seed.
	// +optional
	OwnerChecks *SeedSettingOwnerChecks `json:"ownerChecks,omitempty" protobuf:"bytes,6,opt,name=ownerChecks"`
	// DependencyWatchdog controls certain settings for the dependency-watchdog components deployed in the seed.
	// +optional
	DependencyWatchdog *SeedSettingDependencyWatchdog `json:"dependencyWatchdog,omitempty" protobuf:"bytes,7,opt,name=dependencyWatchdog"`
	// TopologyAwareRouting controls certain settings for topology-aware traffic routing in the seed.
	// See https://github.com/gardener/gardener/blob/master/docs/usage/topology_aware_routing.md.
	// +optional
	TopologyAwareRouting *SeedSettingTopologyAwareRouting `json:"topologyAwareRouting,omitempty" protobuf:"bytes,8,opt,name=topologyAwareRouting"`
}

// SeedSettingExcessCapacityReservation controls the excess capacity reservation for shoot control planes in the seed.
type SeedSettingExcessCapacityReservation struct {
	// Enabled controls whether the excess capacity reservation should be enabled.
	Enabled bool `json:"enabled" protobuf:"bytes,1,opt,name=enabled"`
}

// SeedSettingScheduling controls settings for scheduling decisions for the seed.
type SeedSettingScheduling struct {
	// Visible controls whether the gardener-scheduler shall consider this seed when scheduling shoots. Invisible seeds
	// are not considered by the scheduler.
	Visible bool `json:"visible" protobuf:"bytes,1,opt,name=visible"`
}

// SeedSettingLoadBalancerServices controls certain settings for services of type load balancer that are created in the
// seed.
type SeedSettingLoadBalancerServices struct {
	// Annotations is a map of annotations that will be injected/merged into every load balancer service object.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,1,rep,name=annotations"`
	// ExternalTrafficPolicy describes how nodes distribute service traffic they
	// receive on one of the service's "externally-facing" addresses.
	// Defaults to "Cluster".
	// +optional
	ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicyType `json:"externalTrafficPolicy,omitempty" protobuf:"bytes,2,opt,name=externalTrafficPolicy"`
	// Zones controls settings, which are specific to the single-zone load balancers in a multi-zonal setup.
	// Can be empty for single-zone seeds. Each specified zone has to relate to one of the zones in seed.spec.provider.zones.
	// +optional
	Zones []SeedSettingLoadBalancerServicesZones `json:"zones,omitempty" protobuf:"bytes,3,rep,name=zones"`
}

// SeedSettingLoadBalancerServicesZones controls settings, which are specific to the single-zone load balancers in a
// multi-zonal setup.
type SeedSettingLoadBalancerServicesZones struct {
	// Name is the name of the zone as specified in seed.spec.provider.zones.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Annotations is a map of annotations that will be injected/merged into the zone-specific load balancer service object.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,2,rep,name=annotations"`
	// ExternalTrafficPolicy describes how nodes distribute service traffic they
	// receive on one of the service's "externally-facing" addresses.
	// Defaults to "Cluster".
	// +optional
	ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicyType `json:"externalTrafficPolicy,omitempty" protobuf:"bytes,3,opt,name=externalTrafficPolicy"`
}

// SeedSettingVerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the
// seed.
type SeedSettingVerticalPodAutoscaler struct {
	// Enabled controls whether the VPA components shall be deployed into the garden namespace in the seed cluster. It
	// is enabled by default because Gardener heavily relies on a VPA being deployed. You should only disable this if
	// your seed cluster already has another, manually/custom managed VPA deployment.
	Enabled bool `json:"enabled" protobuf:"bytes,1,opt,name=enabled"`
}

// SeedSettingOwnerChecks controls certain owner checks settings for shoots scheduled on this seed.
type SeedSettingOwnerChecks struct {
	// Enabled controls whether owner checks are enabled for shoots scheduled on this seed. It
	// is enabled by default because it is a prerequisite for control plane migration.
	Enabled bool `json:"enabled" protobuf:"bytes,1,opt,name=enabled"`
}

// SeedSettingDependencyWatchdog controls the dependency-watchdog settings for the seed.
type SeedSettingDependencyWatchdog struct {
	// Endpoint controls the endpoint settings for the dependency-watchdog for the seed.
	// +optional
	Endpoint *SeedSettingDependencyWatchdogEndpoint `json:"endpoint,omitempty" protobuf:"bytes,1,opt,name=endpoint"`
	// Probe controls the probe settings for the dependency-watchdog for the seed.
	// +optional
	Probe *SeedSettingDependencyWatchdogProbe `json:"probe,omitempty" protobuf:"bytes,2,opt,name=probe"`
}

// SeedSettingDependencyWatchdogEndpoint controls the endpoint settings for the dependency-watchdog for the seed.
type SeedSettingDependencyWatchdogEndpoint struct {
	// Enabled controls whether the endpoint controller of the dependency-watchdog should be enabled. This controller
	// helps to alleviate the delay where control plane components remain unavailable by finding the respective pods in
	// CrashLoopBackoff status and restarting them once their dependants become ready and available again.
	Enabled bool `json:"enabled" protobuf:"bytes,1,opt,name=enabled"`
}

// SeedSettingDependencyWatchdogProbe controls the probe settings for the dependency-watchdog for the seed.
type SeedSettingDependencyWatchdogProbe struct {
	// Enabled controls whether the probe controller of the dependency-watchdog should be enabled. This controller
	// scales down the kube-controller-manager of shoot clusters in case their respective kube-apiserver is not
	// reachable via its external ingress in order to avoid melt-down situations.
	Enabled bool `json:"enabled" protobuf:"bytes,1,opt,name=enabled"`
}

// SeedSettingTopologyAwareRouting controls certain settings for topology-aware traffic routing in the seed.
// See https://github.com/gardener/gardener/blob/master/docs/usage/topology_aware_routing.md.
type SeedSettingTopologyAwareRouting struct {
	// Enabled controls whether certain Services deployed in the seed cluster should be topology-aware.
	// These Services are etcd-main-client, etcd-events-client, kube-apiserver, gardener-resource-manager and vpa-webhook.
	Enabled bool `json:"enabled" protobuf:"bytes,1,opt,name=enabled"`
}

// SeedTaint describes a taint on a seed.
type SeedTaint struct {
	// Key is the taint key to be applied to a seed.
	Key string `json:"key" protobuf:"bytes,1,opt,name=key"`
	// Value is the taint value corresponding to the taint key.
	// +optional
	Value *string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
}

const (
	// SeedTaintProtected is a constant for a taint key on a seed that marks it as protected. Protected seeds
	// may only be used by shoots in the `garden` namespace.
	SeedTaintProtected = "seed.gardener.cloud/protected"
)

// SeedVolume contains settings for persistentvolumes created in the seed cluster.
type SeedVolume struct {
	// MinimumSize defines the minimum size that should be used for PVCs in the seed.
	// +optional
	MinimumSize *resource.Quantity `json:"minimumSize,omitempty" protobuf:"bytes,1,opt,name=minimumSize"`
	// Providers is a list of storage class provisioner types for the seed.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	Providers []SeedVolumeProvider `json:"providers,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,2,rep,name=providers"`
}

// SeedVolumeProvider is a storage class provisioner type.
type SeedVolumeProvider struct {
	// Purpose is the purpose of this provider.
	Purpose string `json:"purpose" protobuf:"bytes,1,opt,name=purpose"`
	// Name is the name of the storage class provisioner type.
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`
}

const (
	// SeedBackupBucketsReady is a constant for a condition type indicating that associated BackupBuckets are ready.
	SeedBackupBucketsReady ConditionType = "BackupBucketsReady"
	// SeedBootstrapped is a constant for a condition type indicating that the seed cluster has been
	// bootstrapped.
	SeedBootstrapped ConditionType = "Bootstrapped"
	// SeedExtensionsReady is a constant for a condition type indicating that the extensions are ready.
	SeedExtensionsReady ConditionType = "ExtensionsReady"
	// SeedGardenletReady is a constant for a condition type indicating that the Gardenlet is ready.
	SeedGardenletReady ConditionType = "GardenletReady"
	// SeedSystemComponentsHealthy is a constant for a condition type indicating the system components health.
	SeedSystemComponentsHealthy ConditionType = "SeedSystemComponentsHealthy"
)

// Resource constants for Gardener object types
const (
	// ResourceShoots is a resource constant for the number of shoots.
	ResourceShoots corev1.ResourceName = "shoots"
)
