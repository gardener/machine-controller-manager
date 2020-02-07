package metalgo

import (
	"fmt"
	"strings"
)

const (
	// TagClusterQualifier identifies the cluster
	TagClusterQualifier = "clusterid"
	// TagServiceQualifier identifies the service
	TagServiceQualifier = "service"
	// TagNamespaceQualifier identifies the namespace
	TagNamespaceQualifier = "namespace"

	// TagClusterPrefix the prefix of the tag used to identify a cluster
	TagClusterPrefix = "cluster.metal-pod.io/" + TagClusterQualifier

	// TagServicePrefix the prefix of the tag used to identify services
	TagServicePrefix = TagClusterPrefix + "/" + TagNamespaceQualifier + "/" + TagServiceQualifier

	// TagMachineQualifier identifies the machine
	TagMachineQualifier = "machineid"

	// TagMachinePrefix the prefix of the tag used to identify a machine
	TagMachinePrefix = "metal.metal-pod.io/" + TagMachineQualifier
)

// BuildServiceTag constructs the service tag for the given cluster and service
func BuildServiceTag(clusterID string, namespace, serviceName string) string {
	return fmt.Sprintf("%s/%s/%s", BuildServiceTagClusterPrefix(clusterID), namespace, serviceName)
}

// BuildServiceTagClusterPrefix constructs the prefix of the service tag that identify all services of a cluster
func BuildServiceTagClusterPrefix(clusterID string) string {
	return fmt.Sprintf("%s=%s", TagServicePrefix, clusterID)
}

// TagIsMachine returns true if the given tag is a machinetag.
func TagIsMachine(tag string) bool {
	return strings.HasPrefix(tag, TagMachinePrefix)
}

// TagIsMemberOfCluster returns true of the given tag is a clustertag and clusterID matches.
func TagIsMemberOfCluster(tag, clusterID string) bool {
	if strings.HasPrefix(tag, TagClusterPrefix) {
		parts := strings.Split(tag, "=")
		if len(parts) != 2 {
			return false
		}
		if strings.HasPrefix(parts[1], clusterID) {
			return true
		}
	}
	return false
}
