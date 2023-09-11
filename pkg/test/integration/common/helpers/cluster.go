// Copyright 2023 SAP SE or an SAP affiliate company
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

package helpers

import (
	"context"

	mcmClientset "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	rbac "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Cluster type to hold cluster specific details
type Cluster struct {
	restConfig          *rest.Config
	Clientset           *kubernetes.Clientset
	apiextensionsClient *apiextensionsclientset.Clientset
	McmClient           *mcmClientset.Clientset
	RbacClient          *rbac.RbacV1Client
	KubeConfigFilePath  string
}

// FillClientSets checks whether the cluster is accessible and returns an error if not
func (c *Cluster) FillClientSets() error {
	clientset, err := kubernetes.NewForConfig(c.restConfig)
	if err == nil {
		c.Clientset = clientset
		err = c.ProbeNodes()
		if err != nil {
			return err
		}
		apiextensionsClient, err := apiextensionsclientset.NewForConfig(c.restConfig)
		if err == nil {
			c.apiextensionsClient = apiextensionsClient
		}
		mcmClient, err := mcmClientset.NewForConfig(c.restConfig)
		if err == nil {
			c.McmClient = mcmClient
		}
		rbacClient, err := rbac.NewForConfig(c.restConfig)
		if err == nil {
			c.RbacClient = rbacClient
		}
	}
	return err
}

// NewCluster returns a Cluster struct
func NewCluster(kubeConfigPath string) (c *Cluster, e error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err == nil {
		c = &Cluster{
			KubeConfigFilePath: kubeConfigPath,
			restConfig:         config,
		}
	} else {
		c = &Cluster{}
	}

	return c, err
}

// IsSeed checks whether the cluster is seed of target cluster
func (c *Cluster) IsSeed(target *Cluster) bool {
	/*
		- (Check if the control cluster is a seed cluster
			Try to retrieve the cluster-name (clusters[0].name) from the target kubeconfig passed in.
			 ---- Check if there is any cluster resource available ( means it is a seed cluster ) and see if there is any cluster with name same as to target cluster-name
			 ---- Alternatively check if there is a namespace with same name as that of cluster name found in kube config
			 ---- when both control and target cluster are the same then return false
			 kubectl get clusters -A
			NAME                             AGE
			shoot--landscape--project-shoot   46h
	*/
	controlClusterName, _ := c.ClusterName()
	targetClusterName, _ := target.ClusterName()

	if controlClusterName == targetClusterName {
		return false
	}
	nameSpaces, _ := c.Clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	for _, namespace := range nameSpaces.Items {
		if namespace.Name == targetClusterName {
			return true
		}
	}
	return false
}

// ClusterName retrieves cluster name from the kubeconfig
func (c *Cluster) ClusterName() (string, error) {
	var clusterName string
	config, err := clientcmd.LoadFromFile(c.KubeConfigFilePath)
	if err != nil {
		return clusterName, err
	}
	for contextName, context := range config.Contexts {
		if contextName == config.CurrentContext {
			clusterName = context.Cluster
		}
	}
	return clusterName, err
}

// GetSecretData combines secrets
func (c *Cluster) GetSecretData(machineClassName string, secretRefs ...*v1.SecretReference) (map[string][]byte, error) {
	var secretData map[string][]byte

	for _, secretRef := range secretRefs {
		if secretRef == nil {
			continue
		}

		secretRef, err := c.getSecret(secretRef, machineClassName)
		if err != nil {
			return nil, err
		}

		if secretRef != nil {
			secretData = mergeDataMaps(secretData, secretRef.Data)
		}
	}
	return secretData, nil
}

func mergeDataMaps(in map[string][]byte, maps ...map[string][]byte) map[string][]byte {
	out := make(map[string][]byte)

	for _, m := range append([]map[string][]byte{in}, maps...) {
		for k, v := range m {
			out[k] = v
		}
	}

	return out
}

// getSecret retrieves the kubernetes secret if found
func (c *Cluster) getSecret(ref *v1.SecretReference, MachineClassName string) (*v1.Secret, error) {
	if ref == nil {
		return nil, nil
	}
	secretRef, err := c.Clientset.CoreV1().Secrets(ref.Namespace).Get(context.Background(), ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return secretRef, err
}
