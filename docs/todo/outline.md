# Machine Controller Manager
CORE -- ./machine-controller-manager(provider independent)
Out of tree : Machine controller (provider specific)
MCM is a set controllers:

*  Machine Deployment Controller

*  Machine  Set Controller

*  Machine  Controller

*  Machine  Safety  Controller

## Questions and refactoring Suggestions

### Refactoring
| Statement | FilePath | Status |
| -- | -- | -- |
| ConcurrentNodeSyncs”  bad  name  -  nothing  to  do  with  node  syncs  actually. <br /> If  its  value  is  ’10’  then  it  will  start  10  goroutines  (workers)  per  resource  type  (machine,  machinist,  machinedeployment,  provider-specific-class,  node  -  study  the  different  resource  types. | cmd/machine-controller-manager/app/options/options.go | pending |
|LeaderElectionConfiguration  is  very  similar  to  the  one  present  in  “client-go/tools/leaderelection/leaderelection.go”  -  can  we  simply  used  the  one  in  client-go  instead  of  defining  again?| pkg/options/types.go - MachineControllerManagerConfiguration | pending |
|Have  all  userAgents  as  constant.  Right  now  there  is  just  one. | cmd/app/controllermanager.go | pending |
|Shouldn’t  run  function  be  defined  on  MCMServer  struct  itself?|cmd/app/controllermanager.go | pending |
| clientcmd.BuildConfigFromFlags  fallsback  to  inClusterConfig  which  will  surely  not  work  as  that  is  not  the  target.  Should  it  not  check  and  exit  early? | cmd/app/controllermanager.go - run Function | pending |
| A  more  direct  way  to  create  an  in  cluster  config  is  using  `k8s.io/client-go/rest`  ->  rest.InClusterConfig  instead  of  using  clientcmd.BuildConfigFromFlags  passing  empty  arguments  and  depending  upon  the  implementation  to  fallback  to  creating  a  inClusterConfig.  If  they  change  the  implementation  that  you  get  affected. | cmd/app/controllermanager.go - run Function | pending |
|Introduce  a  method  on  MCMServer  which  gets  a  target  KubeConfig  and  controlKubeConfig  or  alternatively  which  creates  respective  clients.| cmd/app/controllermanager.go - run Function | pending |
|Why  can’t  we  use  Kubernetes.NewConfigOrDie  also  for  kubeClientControl?| cmd/app/controllermanager.go - run Function | pending |
| I  do  not  see  any  benefit  of  client  builders  actually.  All  you  need  to  do  is  pass  in  a  config  and  then  directly  use  client-go  functions  to  create  a  client. | cmd/app/controllermanager.go - run Function | pending |
| Function:  getAvailableResources  -  rename  this  to  getApiServerResources  | cmd/app/controllermanager.go | pending |
|Move  the  method  which  waits  for  API  server  to  up  and  ready  to  a  separate  method  which  returns  a  discoveryClient  when  the  API  server  is  ready. | cmd/app/controllermanager.go - getAvailableResources function | pending |
| Many  methods  in  client-go  used  are  now  deprecated.  Switch  to  the  ones  that  are  now  recommended  to  be  used  instead. | cmd/app/controllermanager.go - startControllers | pending |
| This  method  needs  a  general  overhaul | cmd/app/controllermanager.go - startControllers | pending |
| If  the  design  is  influenced/copied  from  KCM  then  its  very  different.  There  are  different  controller  structs  defined  for  deployment,  replicaset  etc  which  makes  the  code  much  more  clearer.  You  can  see  “kubernetes/cmd/kube-controller-manager/apps.go”  and  then  follow  the  trail  from  there.  -  agreed  needs  to  be  changed  in  future  (if  time  permits) | pkg/controller/controller.go | pending |
| I  am  not  sure  why  “MachineSetControlInterface”,  “RevisionControlInterface”,  “MachineControlInterface”,  “FakeMachineControl”  are  defined  in  this  file? | pkg/controller/controller_util.go | pending |
| `IsMachineActive`  -  combine  the  first  2  conditions  into  one  with  OR. | pkg/controller/controller_util.go | pending |
| Minor  change  -  correct  the  comment,  first  word  should  always  be  the  method  name.  Currently  none  of  the  comments  have  correct  names. | pkg/controller/controller_util.go | pending |
| There  are  too  many  deep  copies  made.  What  is  the  need  to  make  another  deep  copy  in  this  method?  You  are  not  really  changing  anything  here. | pkg/controller/deployment.go - updateMachineDeploymentFinalizers | pending |
| Why  can't  these  validations  be  done  as  part  of  a  validating  webhook? | pkg/controller/machineset.go - reconcileClusterMachineSet | pending |
| Small  change  to  the  following  `if`  condition.  `else  if`  is  not  required  a  simple  `else`  is  sufficient. [Code1](#1.1-code1)
| pkg/controller/machineset.go -  reconcileClusterMachineSet | pending |
|  Why  call  these  `inactiveMachines`,  these  are  live  and  running  and  therefore  active. | pkg/controller/machineset.go - terminateMachines | pending |

### Clarification

| Statement | FilePath | Status |
| -- | -- | -- |
| Why  are  there  2  versions  -  internal  and  external  versions? | General | pending |
| Safety  controller  freezes  MCM  controllers  in  the  following  cases:         <br /> *  Num  replicas  go  beyond  a  threshold  (above  the  defined  replicas)         <br /> *  Target  API  service  is  not  reachable <br /> There  seems  to  be  an  overlap  between  DWD  and  MCM  Safety  controller.  In  the  meltdown  scenario  why  is  MCM  being  added  to  DWD,  you  could  have  used  Safety  controller  for  that. | General | pending |
| All  machine  resources  are  v1alpha1  -  should  we  not  promote  it  to  beta.  V1alpha1  has  a  different  semantic  and  does  not  give  any  confidence  to  the  consumers. | cmd/app/controllermanager.go | pending |
| Shouldn’t  controller  manager  use  context.Context  instead  of  creating  a  stop  channel?  -  Check  if  signals  (`os.Interrupt`  and  `SIGTERM`  are  handled  properly.  Do  not  see  code  where  this  is  handled  currently.) | cmd/app/controllermanager.go | pending |
| What  is  the  rationale  behind  a  timeout  of  10s?  If  the  API  server  is  not  up,  should  this  not  just  block  as  it  can  anyways  not  do  anything.  Also,  if  there  is  an  error  returned  then  you  exit  the  MCM  which  does  not  make  much  sense  actually  as  it  will  be  started  again  and  you  will  again  do  the  poll  for  the  API  server  to  come  back  up.  Forcing  an  exit  of  MCM  will  not  have  any  impact  on  the  reachability  of  the  API  server  in  anyway  so  why  exit? | cmd/app/controllermanager.go - getAvailableResources | pending |
| There  is  a  very  weird  check  -  availableResources[machineGVR]  \|\|  availableResources[machineSetGVR]  \|\| availableResources[machineDeploymentGVR] <br />\*  Shouldn’t  this  be  conjunction  instead  of  disjunction? <br />\*  What  happens  if  you  do  not  find  one  or  all  of  these  resources? <br /> Currently  an  error  log  is  printed  and  nothing  else  is  done. MCM can be used outside gardener context where consumers can directly create MachineClass and Machine and not create MachineSet / Maching Deployment. There is no distinction made between context (gardener or outside-gardener). | cmd/app/controllermanager.go - StartControllers | pending |
| Instead  of  having  an  empty  select  {}  to  block  forever,  isn’t  it  better  to  wait  on  the  stop  channel? | cmd/app/controllermanager.go - StartControllers | pending |
| Do  we  need  provider  specific  queues  and  syncs  and  listers | pkg/controller/controller.go | pending |
| Why  are  resource  types  prefixed  with  “Cluster”?  -  not  sure  ,  check  PR | pkg/controller/controller.go | pending |
| When  will  forgetAfterSuccess  be  false  and  why?  -  as  per  the  current  code  this  is  never  the  case.  -  Himanshu  will  check | cmd/app/controllermanager.go - createWorker | pending |
| What  is  the  use  of  “ExpectationsInterface”  and  “UIDTrackingContExpectations”? <br />*  All  expectations  related  code  should  be  in  its  own  file  “expectations.go”  and  not  in  this  file. | pkg/controller/controller_util.go | pending |
| Why  do  we  not  use  lister  but  directly  use  the  controlMachingClient  to  get  the  deployment?  Is  it  because  you  want  to  avoid  any  potential  delays  caused  by  update  of  the  local  cache  held  by  the  informer  and  accessed  by  the  lister?  What  is  the  load  on  API  server  due  to  this? | pkg/controller/deployment.go -  reconcileClusterMachineDeployment | pending |
| Why  is  this  conversion  needed? [code2](#1.2-code2) | pkg/controller/deployment.go -  reconcileClusterMachineDeployment | pending |
| A  deep  copy  of  `machineDeployment`  is  already  passed  and  within  the  function  another  deepCopy  is  made.  Any  reason  for  it? | pkg/controller/deployment.go - addMachineDeploymentFinalizers| pending |
| What  is  an  `Status.ObservedGeneration`?  <br /> **Read  more  about  generations  and  observedGeneration  at: <br />  https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#metadata <br />*  https://alenkacz.medium.com/kubernetes-operator-best-practices-implementing-observedgeneration-250728868792 <br /> Ideally the update to the `ObservedGeneration` should only be made after successful reconciliation and not before. I see that this is just copied from `deployment_controller.go` as is | pkg/controller/deployment.go -  reconcileClusterMachineDeployment | pending |
| Why  and  when  will  a  `MachineDeployment`  be  marked  as  frozen  and  when  will  it  be  un-frozen? | pkg/controller/deployment.go -  reconcileClusterMachineDeployment | pending |
| Shoudn't  the  validation  of  the  machine  deployment  be  done  during  the  creation  via  a  validating  webhook  instead  of  allowing  it  to  be  stored  in  etcd  and  then  failing  the  validation  during  sync?  I  saw  the  checks  and  these  can  be  done  via  validation  webhook. | pkg/controller/deployment.go -  reconcileClusterMachineDeployment | pending |
| RollbackTo has been marked as deprecated. What is the replacement? [code3](#1.3-code3) | pkg/controller/deployment.go -  reconcileClusterMachineDeployment | pending |
| What  is  the  max  machineSet  deletions  that  you  could  process  in  a  single  run?  The  reason  for  asking  this  question  is  that  for  every  machineSetDeletion  a  new  goroutine  spawned. <br />*  Is  the  `Delete`  call  a  synchrounous  call?  Which  means  it  blocks  till  the  machineset  deletion  is  triggered  which  then  also  deletes  the  machines  (due  to  cascade-delete  and  blockOwnerDeletion=  true)?| pkg/controller/deployment.go - terminateMachineSets | pending |
| If  there  are  validation  errors  or  error  when  creating  label  selector  then  a  nil  is  returned.  In  the  worker  reconcile  loop  if  the  return  value  is  nil  then  it  will  remove  it  from  the  queue  (forget  +  done).  What  is  the  way  to  see  any  errors?  Typically  when  we  describe  a  resource  the  errors  are  displayed.  Will  these  be  displayed  when  we  discribe  a  `MachineDeployment`? | pkg/controller/deployment.go - reconcileClusterMachineSet | pending |
| If  an  error  is  returned  by  `updateMachineSetStatus`  and  it  is  `IsNotFound`  error  then  returning  an  error  will  again  queue  the  `MachineSet`.  Is  this  desired  as  `IsNotFound`  indicates  the  `MachineSet`  has  been  deleted  and  is  no  longer  there? | pkg/controller/deployment.go -  reconcileClusterMachineSet | pending |
| is  `machineControl.DeleteMachine`  a  synchronous  operation  which  will  wait  till  the  machine  has  been  deleted?  Also  where  is  the  `DeletionTimestamp`  set  on  the  `Machine`?  Will  it  be  automatically  done  by  the  API  server? | pkg/controller/deployment.go - prepareMachineForDeletion | pending |

### Bugs/Enhancements

| Statement + TODO | FilePath | Status |
| -- | -- | -- |
|This  defines  QPS  and  Burst  for  its  requests  to  the  KAPI.  Check  if  it  would  make  sense  to  explicitly  define  a  FlowSchema  and  PriorityLevelConfiguration  to  ensure  that  the  requests  from  this  controller  are  given  a  well-defined  preference.  What  is  the  rational  behind  deciding  these  values? | pkg/options/types.go - MachineControllerManagerConfiguration | pending |
| In  function  “validateMachineSpec”  fldPath  func  parameter  is  never  used. | pkg/apis/machine/validation/machine.go | pending |
| If  there  is  an  update  failure  then  this  method  recursively  calls  itself  without  any  sort  of  delays  which  could  lead  to  a  LOT  of  load  on  the  API  server.  (opened:  https://github.com/gardener/machine-controller-manager/issues/686) | pkg/controller/deployment.go - updateMachineDeploymentFinalizers | pending |
| We  are  updating  `filteredMachines`  by  invoking  `syncMachinesNodeTemplates`,  `syncMachinesConfig`  and  `syncMachinesClassKind`  but  we  do  not  create  any  deepCopy  here.  Everywhere  else  the  general  principle  is  when  you  mutate  always  make  a  deepCopy  and  then  mutate  the  copy  instead  of  the  original  as  a  lister  is  used  and  that  changes  the  cached  copy. <br /> `Fix`:  `SatisfiedExpectations`  check  has  been  commented  and  there  is  a  TODO  there  to  fix  it.  Is  there  a  PR  for  this? | pkg/controller/machineset.go - reconcileClusterMachineSet | pending |


Code references
# 1.1 code1 
```go
       if machineSet.DeletionTimestamp == nil {
        
        		// manageReplicas is the core machineSet method where scale up/down occurs
        
        		// It is not called when deletion timestamp is set
        
        		manageReplicasErr = c.manageReplicas(ctx, filteredMachines, machineSet)
        
        ​
        
        	} else if machineSet.DeletionTimestamp != nil { 
        
            //FIX: change this to simple else without the if
```

# 1.2 code2
```go
    defer dc.enqueueMachineDeploymentAfter(deployment, 10*time.Minute)
    
    *  `Clarification`:  Why  is  this  conversion  needed?
    
    err = v1alpha1.Convert_v1alpha1_MachineDeployment_To_machine_MachineDeployment(deployment, internalMachineDeployment, nil)
```

# 1.3 code3
```go

// rollback is not re-entrant in case the underlying machine sets are updated with a new

	// revision so we should ensure that we won't proceed to update machine sets until we

	// make sure that the deployment has cleaned up its rollback spec in subsequent enqueues.

	if d.Spec.RollbackTo != nil {

		return dc.rollback(ctx, d, machineSets, machineMap)

	}

```
