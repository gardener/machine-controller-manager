package helpers

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	mcmscheme "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	scheme "k8s.io/client-go/kubernetes/scheme"
)

// ParseK8sYaml reads a yaml file and parses it based on the scheme
func ParseK8sYaml(filepath string) ([]runtime.Object, []*schema.GroupVersionKind, error) {
	fileR, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, nil, err
	}

	acceptedK8sTypes := regexp.MustCompile(`(Role|ClusterRole|RoleBinding|ClusterRoleBinding|ServiceAccount|CustomResourceDefinition|kind: Deployment)`)
	acceptedMCMTypes := regexp.MustCompile(`(MachineClass|Machine)`)
	fileAsString := string(fileR[:])
	sepYamlfiles := strings.Split(fileAsString, "---\n")
	retObj := make([]runtime.Object, 0, len(sepYamlfiles))
	retKind := make([]*schema.GroupVersionKind, 0, len(sepYamlfiles))
	var retErr error
	crdRegexp, _ := regexp.Compile("CustomResourceDefinition")
	for _, f := range sepYamlfiles {
		if f == "\n" || f == "" {
			// ignore empty cases
			continue
		}

		// isExist, err := regexp.Match("CustomResourceDefinition", []byte(f))
		if crdRegexp.Match([]byte(f)) {
			decode := apiextensionsscheme.Codecs.UniversalDeserializer().Decode
			obj, groupVersionKind, err := decode([]byte(f), nil, nil)
			if err != nil {
				log.Println(fmt.Sprintf("Error while decoding YAML object. Err was: %s", err))
				retErr = err
				continue
			}
			if !acceptedK8sTypes.MatchString(groupVersionKind.Kind) {
				log.Printf(
					"The custom-roles configMap contained K8s object types which are not supported! Skipping object with type: %s",
					groupVersionKind.Kind)
			} else {
				retKind = append(retKind, groupVersionKind)
				retObj = append(retObj, obj)
			}
		} else if acceptedK8sTypes.Match([]byte(f)) {
			decode := scheme.Codecs.UniversalDeserializer().Decode
			obj, groupVersionKind, err := decode([]byte(f), nil, nil)
			if err != nil {
				log.Println(fmt.Sprintf("Error while decoding YAML object. Err was: %s", err))
				retErr = err
				continue
			}
			retKind = append(retKind, groupVersionKind)
			retObj = append(retObj, obj)

		} else {
			decode := mcmscheme.Codecs.UniversalDeserializer().Decode
			obj, groupVersionKind, err := decode([]byte(f), nil, nil)
			if err != nil {
				log.Println(fmt.Sprintf("Error while decoding YAML object. Err was: %s", err))
				retErr = err
				continue
			}
			if !acceptedMCMTypes.MatchString(groupVersionKind.Kind) {
				log.Printf(
					"The custom-roles configMap contained K8s object types which are not supported! Skipping object with type: %s",
					groupVersionKind.Kind)
			} else {
				retKind = append(retKind, groupVersionKind)
				retObj = append(retObj, obj)
			}
		}
	}
	return retObj, retKind, retErr
}

// applyFile uses yaml to create resources in kubernetes
func (c *Cluster) applyFile(filePath string, namespace string) error {
	ctx := context.Background()
	runtimeobj, kind, err := ParseK8sYaml(filePath)
	if err == nil {
		for key, obj := range runtimeobj {
			switch kind[key].Kind {
			case "CustomResourceDefinition":
				crd := obj.(*apiextensionsv1.CustomResourceDefinition)

				if _, err := c.apiextensionsClient.
					ApiextensionsV1().
					CustomResourceDefinitions().
					Create(ctx, crd, metav1.CreateOptions{}); err != nil {
					if !strings.Contains(err.Error(), "already exists") {
						return err
					}
				}
				err = c.checkEstablished(crd.Name)
				if err != nil {
					log.Printf("%s crd can not be established because of an error\n", crd.Name)
					return err
				}
			case "MachineClass":
				resource := obj.(*v1alpha1.MachineClass)

				if _, err := c.McmClient.MachineV1alpha1().
					MachineClasses(namespace).
					Create(ctx, resource, metav1.CreateOptions{}); err != nil {
					return err
				}
			case "Machine":
				resource := obj.(*v1alpha1.Machine)

				if _, err := c.McmClient.MachineV1alpha1().
					Machines(namespace).
					Create(ctx, resource, metav1.CreateOptions{}); err != nil {
					return err
				}
			case "MachineDeployment":
				resource := obj.(*v1alpha1.MachineDeployment)

				if _, err := c.McmClient.MachineV1alpha1().
					MachineDeployments(namespace).
					Create(ctx, resource, metav1.CreateOptions{}); err != nil {
					return err
				}
			case "Deployment":
				resource := obj.(*v1.Deployment)

				if _, err := c.Clientset.AppsV1().
					Deployments(namespace).
					Create(ctx, resource, metav1.CreateOptions{}); err != nil {
					return err
				}
			case "ClusterRoleBinding":
				resource := obj.(*rbacv1.ClusterRoleBinding)
				for i := range resource.Subjects {
					resource.Subjects[i].Namespace = namespace
				}

				if _, err := c.Clientset.RbacV1().
					ClusterRoleBindings().
					Create(ctx, resource, metav1.CreateOptions{}); err != nil {
					return err
				}
			case "ClusterRole":
				resource := obj.(*rbacv1.ClusterRole)

				if _, err := c.Clientset.RbacV1().
					ClusterRoles().
					Create(ctx, resource, metav1.CreateOptions{}); err != nil {
					return err
				}
			}
		}
	} else {
		return err
	}
	return nil
}

// checkEstablished uses the specified name to check if it is established
func (c *Cluster) checkEstablished(crdName string) error {
	err := wait.Poll(500*time.Millisecond, 60*time.Second, func() (bool, error) {
		crd, err := c.apiextensionsClient.
			ApiextensionsV1().
			CustomResourceDefinitions().
			Get(
				context.Background(),
				crdName,
				metav1.GetOptions{},
			)
		if err != nil {
			return false, err
		}
		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiextensionsv1.Established:
				if cond.Status == apiextensionsv1.ConditionTrue {
					// log.Printf("crd %s is established/ready\n", crdName)
					return true, err
				}
			case apiextensionsv1.NamesAccepted:
				if cond.Status == apiextensionsv1.ConditionFalse {
					log.Printf("Name conflict: %v\n", cond.Reason)
					log.Printf("Naming Conflict with created CRD %s\n", crdName)
				}
			}
		}
		return false, err
	})
	return err
}

// ApplyFiles invokes ApplyFile on a each file
func (c *Cluster) ApplyFiles(source string, namespace string) error {
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsDir() {
			log.Printf("%s is a directory.\n", path)
		} else if info.Mode().IsRegular() {
			err := c.applyFile(path, namespace)
			if err != nil {
				//Ignore error if it says the crd already exists
				if !strings.Contains(err.Error(), "already exists") {
					log.Printf("Failed to apply yaml file %s", path)
					return err
				}
			}
			log.Printf("file %s has been successfully applied to cluster", path)
		}
		return nil
	})
	return err
}

// deleteResource uses yaml to delete resources in kubernetes
func (c *Cluster) deleteResource(filePath string, namespace string) error {
	ctx := context.Background()
	runtimeobj, kind, err := ParseK8sYaml(filePath)
	if err == nil {
		for key, obj := range runtimeobj {
			switch kind[key].Kind {
			case "CustomResourceDefinition":
				crd := obj.(*apiextensionsv1.CustomResourceDefinition)

				if err := c.apiextensionsClient.ApiextensionsV1().
					CustomResourceDefinitions().
					Delete(
						ctx,
						crd.Name,
						metav1.DeleteOptions{},
					); err != nil {

					if strings.Contains(err.Error(), " not found") {
					} else {
						return err
					}
				}
			case "MachineClass":
				resource := obj.(*v1alpha1.MachineClass)
				err := c.McmClient.
					MachineV1alpha1().
					MachineClasses(namespace).
					Delete(
						ctx,
						resource.Name,
						metav1.DeleteOptions{},
					)
				if err != nil {
					return err
				}
			case "MachineDeployment":
				resource := obj.(*v1alpha1.MachineDeployment)
				err := c.McmClient.
					MachineV1alpha1().
					MachineDeployments(namespace).
					Delete(
						ctx,
						resource.Name,
						metav1.DeleteOptions{},
					)
				if err != nil {
					return err
				}
			case "Machine":
				resource := obj.(*v1alpha1.Machine)
				err := c.McmClient.
					MachineV1alpha1().
					Machines(namespace).
					Delete(
						ctx,
						resource.Name,
						metav1.DeleteOptions{},
					)
				if err != nil {
					return err
				}
			case "Deployment":
				resource := obj.(*v1.Deployment)
				err := c.Clientset.
					AppsV1().
					Deployments(namespace).
					Delete(
						ctx,
						resource.Name,
						metav1.DeleteOptions{},
					)
				if err != nil {
					return err
				}
			case "ClusterRoleBinding":
				resource := obj.(*rbacv1.ClusterRoleBinding)
				for i := range resource.Subjects {
					resource.Subjects[i].Namespace = namespace
				}
				if err := c.Clientset.RbacV1beta1().
					ClusterRoleBindings().
					Delete(
						ctx,
						resource.Name,
						metav1.DeleteOptions{},
					); err != nil {
					return err
				}
			case "ClusterRole":
				resource := obj.(*rbacv1.ClusterRole)
				if err := c.Clientset.RbacV1beta1().
					ClusterRoles().
					Delete(
						ctx,
						resource.Name,
						metav1.DeleteOptions{},
					); err != nil {
					return err
				}
			}
		}
	} else {
		return err
	}
	return nil
}

// DeleteResources invokes DeleteResource on a each file
func (c *Cluster) DeleteResources(source string, namespace string) error {
	return filepath.Walk(
		source,
		func(path string, info os.FileInfo, err error) error {
			if info.Mode().IsDir() {
				log.Printf("%s is a directory.\n", path)
			} else if info.Mode().
				IsRegular() {
				if err := c.deleteResource(path, namespace); err != nil {
					//Ignore error if it says the crd already exists
					if !strings.Contains(err.Error(), " not found") {
						log.Printf("Failed to remove resource %s", path)
						return err
					}
				}
				log.Printf("resource %s has been successfully removed from the cluster", path)
			}
			return nil
		},
	)
}
