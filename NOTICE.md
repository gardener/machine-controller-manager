Machine Controller Manager.  
Copyright 2017 The Gardener Authors.  
[Apache 2 license](./LICENSE.md ).   

## Seed Source

The source code of this component was seeded based on a copy of the following files from kubernetes/kubernetes. 

Kubernetes.  
https://github.com/kubernetes/kubernetes/tree/release-1.8.  
Copyright 2017 The Kubernetes Authors.   
Apache 2 license (https://github.com/kubernetes/kubernetes/blob/release-1.8/LICENSE )

Release: 1.8.   
Commit-ID: 682da6ea1fd7a8b471d84c83b17c5239ded056d5.    
Commit-Message:  Add/Update CHANGELOG-1.8.md for v1.8.6.     
To the left are the list of copied files -> and to the right the current location they are at.  

	cmd/kube-controller-manager/app/controllermanager.go -> cmd/machine-controller-manager/app/controllermanager.go
	cmd/kube-controller-manager/app/options/options.go -> cmd/machine-controller-manager/app/options/options.go
	cmd/kube-controller-manager/controller_manager.go -> cmd/machine-controller-manager/controller_manager.go
	pkg/controller/deployment/deployment_controller.go -> pkg/controller/deployment_controller.go
	pkg/controller/deployment/util/replicaset_util.go -> pkg/controller/deployment_machineset_util.go
	pkg/controller/deployment/progress.go -> pkg/controller/deployment_progress.go
	pkg/controller/deployment/recreate.go -> pkg/controller/deployment_recreate.go
	pkg/controller/deployment/rollback.go -> pkg/controller/deployment_rollback.go
	pkg/controller/deployment/rolling.go -> pkg/controller/deployment_rolling.go
	pkg/controller/deployment/sync.go -> pkg/controller/deployment_sync.go
	pkg/controller/deployment/util/deployment_util.go -> pkg/controller/deployment_util.go
	pkg/controller/deployment/util/hash_test.go -> pkg/controller/hasttest.go
	pkg/controller/deployment/util/pod_util.go -> pkg/controller/machine_util.go
	pkg/controller/replicaset/replica_set.go -> pkg/controller/machineset.go
	pkg/controller/deployment/util/replicaset_util.go -> pkg/controller/machineset_util.go

## Dependencies

The machine-controller-manager includes the following components

OAI Object Model.   
https://github.com/go-openapi/spec.  
Copyright 2015 go-swagger maintainers.  
Apache 2 license (https://github.com/go-openapi/spec/blob/master/LICENSE ).  

Glog.  
https://github.com/golang/glog.  
Copyright 2013 Google Inc. All Rights Reserved.  
Apache 2 license (https://github.com/golang/glog/blob/master/LICENSE ).  

Prometheus Go Client Library.  
https://github.com/prometheus/client_golang.  
Copyright 2015 The Prometheus Authors.  
Apache 2 license (https://github.com/prometheus/client_golang/blob/master/LICENSE ).  

Pflag.   
https://github.com/spf13/pflag.  
Copyright (c) 2012 Alex Ogier. Copyright (c) 2012 The Go Authors.   
BSD 3-clause "New" or "Revised" License (https://github.com/spf13/pflag/blob/master/LICENSE ).  

API.  
https://github.com/kubernetes/api/.  
Copyright 2017 The Kubernetes Authors.   
Apache 2 license (https://github.com/kubernetes/api/blob/master/LICENSE ).  

APIMachinery.  
https://github.com/kubernetes/apimachinery.  
Copyright 2017 The Kubernetes Authors.      
Apache 2 license (https://github.com/kubernetes/apimachinery/blob/master/LICENSE ).  

APIServer.  
https://github.com/kubernetes/apiserver.  
Copyright 2017 The Kubernetes Authors.   
Apache 2 license (https://github.com/kubernetes/apiserver/blob/master/LICENSE ).  

Client-go.  
https://github.com/kubernetes/client-go.  
Copyright 2017 The Kubernetes Authors.   
Apache 2 license (https://github.com/kubernetes/client-go/blob/master/LICENSE ).  

Code-generator.  
https://github.com/kubernetes/code-generator.  
Copyright 2017 The Kubernetes Authors.   
Apache 2 license (https://github.com/kubernetes/code-generator/blob/master/LICENSE ).  

Gengo.  
https://github.com/kubernetes/gengo.  
Copyright 2017 The Kubernetes Authors.   
Apache 2 license (https://github.com/kubernetes/gengo/blob/master/LICENSE ).   

Kube OpenAPI.  
https://github.com/kubernetes/kube-openapi.  
Copyright 2017 The Kubernetes Authors.   
Apache 2 license (https://github.com/kubernetes/kube-openapi/blob/master/LICENSE ).  

Google Cloud Go SDK.  
https://cloud.google.com/compute/docs/api/libraries#google_apis_go_client_library.  
Copyright (c) 2011 Google Inc. All rights reserved.  
BSD-3 license (https://github.com/google/google-api-go-client/blob/master/LICENSE)  

Gophercloud.  
https://github.com/gophercloud/gophercloud.  
Copyright 2012-2013 Rackspace, Inc.  
Apache 2 license (https://github.com/gophercloud/gophercloud/blob/master/LICENSE)  
