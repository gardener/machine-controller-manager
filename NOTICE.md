Node controller manager.  
Copyright 2017 The Gardener Authors.  
Apache 2 license (https://github.wdf.sap.corp/kubernetes/node-controller-manager/blob/master/LICENCE.md ).  

The source code of this component was seeded based on a copy of the following files from kubernetes/kubernetes.   
Release: 1.8.   
Commit-ID: 682da6ea1fd7a8b471d84c83b17c5239ded056d5.  
Commit-Message:  Add/Update CHANGELOG-1.8.md for v1.8.6.   
Link: https://github.com/kubernetes/kubernetes/tree/release-1.8.  

To the left are the list of copied files -> and to the right the current location they are at.  

	cmd/kube-controller-manager/app/controllermanager.go -> cmd/node-controller-manager/app/controllermanager.go
	cmd/kube-controller-manager/app/options/options.go -> cmd/node-controller-manager/app/options/options.go
	cmd/kube-controller-manager/controller_manager.go -> cmd/node-controller-manager/controller_manager.go
	pkg/controller/deployment/deployment_controller.go -> pkg/controller/deployment_controller.go
	pkg/controller/deployment/util/instanceset_util.go -> pkg/controller/deployment_instanceset_util.go
	pkg/controller/deployment/progress.go -> pkg/controller/deployment_progress.go
	pkg/controller/deployment/recreate.go -> pkg/controller/deployment_recreate.go
	pkg/controller/deployment/rollback.go -> pkg/controller/deployment_rollback.go
	pkg/controller/deployment/rolling.go -> pkg/controller/deployment_rolling.go
	pkg/controller/deployment/sync.go -> pkg/controller/deployment_sync.go
	pkg/controller/deployment/util/deployment_util.go -> pkg/controller/deployment_util.go
	pkg/controller/deployment/util/hash_test.go -> pkg/controller/hasttest.go
	pkg/controller/deployment/util/pod_util.go -> pkg/controller/instance_util.go
	pkg/controller/replicaset/replica_set.go -> pkg/controller/instanceset.go
	pkg/controller/deployment/util/replicaset_util.go -> pkg/controller/instanceset_util.go