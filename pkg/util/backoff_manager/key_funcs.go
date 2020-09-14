package backoff_manager

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func MachineSetKeyedMachine(i interface{}) (key string) {
	object := i.(metav1.Object)

	for _, owner := range object.GetOwnerReferences() {
		if owner.Kind == "MachineSet" {
			key = owner.Name
		}
	}

	return
}
