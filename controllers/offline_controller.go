/*

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

package controllers

import (
	"context"
	colocationv1 "github.com/YunWang/colocation/api/v1"
	"github.com/YunWang/colocation/pkg/cache"
	"github.com/YunWang/colocation/pkg/utils"
	"github.com/go-logr/logr"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const (
	OfflineFinalizer  = "offline.colocation.cmyun.io"
	OfflineAPIVersion = "colocation.cmyun.io/v1"
	OfflineKind = "Offline"
)


// OfflineReconciler reconciles a Offline object
type OfflineReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Cache *cache.Cache
}

// +kubebuilder:rbac:groups=colocation.cmyun.io,resources=offlines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=colocation.cmyun.io,resources=offlines/status,verbs=get;update;patch

func (r *OfflineReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("offline", req.NamespacedName)

	off:=&colocationv1.Offline{}
	if err:=r.Client.Get(ctx,req.NamespacedName,off);err!=nil{
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	//create
	if off.ObjectMeta.CreationTimestamp.IsZero() {
		//createTimestamp
		off.ObjectMeta.CreationTimestamp=v1.Now()
		//add to scheduling queue
		queueName:=utils.GetOfflineQueueName(off)
		queue:=r.Cache.Get(queueName)
		if err:=queue.AddSchedulingQ(off);err!=nil{
			return ctrl.Result{},err
		}
		//finalizer for delete
		off.Finalizers=append(off.ObjectMeta.Finalizers, OfflineFinalizer)
		off.Status.Phase=colocationv1.OfflinePendingPhase
	}
	podList:=&v12.PodList{}
	if err:=r.Client.List(ctx,podList,client.InNamespace(req.Namespace),client.MatchingLabelsSelector{labels.SelectorFromSet(off.Spec.Selector.MatchLabels)});err!=nil{
		return ctrl.Result{},err
	}

	//delete
	if !off.DeletionTimestamp.IsZero() {
		//set deletetimestamp
		for _,pod := range podList.Items {
			pod.ObjectMeta.DeletionTimestamp=&v1.Time{Time:time.Now()}
		}
		//delete offline from queue
		queueName:=utils.GetOfflineQueueName(off)
		queue:=r.Cache.Get(queueName)
		if err:=queue.Delete(off);err!=nil{
			return ctrl.Result{},err
		}
		//delete finalizer
		for index,finalizer := range off.Finalizers {
			if finalizer==OfflineFinalizer {
				off.Finalizers= append(off.Finalizers[:index], off.Finalizers[index+1:]...)
			}
		}
	}

	//update
	minGang:=off.Spec.MinGang
	succeedNum:=off.Status.PodSucceeded
	pendingNum:=off.Status.PodPending
	runningNum:=off.Status.PodRunning
	failedNum:=off.Status.PodFailed
	unknownNum:=off.Status.PodUnknown
	if succeedNum==0&&pendingNum==0&&runningNum==0&&failedNum==0&&unknownNum==0{
		off.Status.Phase=colocationv1.OfflinePendingPhase
	}else{
		if failedNum !=0 || unknownNum!=0{
			off.Status.Phase=colocationv1.OfflineFailedPhase
		}else{
			if succeedNum+runningNum<minGang{
				off.Status.Phase=colocationv1.OfflineSchedulingPhase
			}else if succeedNum+runningNum>=minGang {
				if runningNum==0 && pendingNum==0 && succeedNum>0{
					off.Status.Phase=colocationv1.OfflineSucceededPhase
				}else{
					off.Status.Phase=colocationv1.OfflineRunningPhase
				}
			}
		}
	}

	//update queue
	if off.Status.Phase==colocationv1.OfflineFailedPhase {
		r.Cache.AddToUnSchedulableQ(off)
	}else if off.Status.Phase==colocationv1.OfflineRunningPhase{
		queueName:=off.Spec.Queue
		queue:=r.Cache.Get(queueName)
		currentSchedulingOffline:=queue.Scheduling()
		if currentSchedulingOffline== Key(off){
			//scheduling next offline
			queue.Delete(off)
			if queue.Len()==0{
				r.Cache.Delete(queueName)
			}else{
				//TODO start offline
				//nextOff,err:=queue.Pop()
				//if err!=nil {
				//	return ctrl.Result{},err
				//}
				//create next offline

			}
		}

	}

	oldOff:=&colocationv1.Offline{}
	if err:=r.Get(ctx,types.NamespacedName{Namespace:off.Namespace,Name:off.Name},oldOff);err!=nil{
		return ctrl.Result{},err
	}
	if !reflect.DeepEqual(oldOff.Status,off.Status) || !reflect.DeepEqual(oldOff.ObjectMeta,off.ObjectMeta){
		oldOff.ObjectMeta=off.ObjectMeta
		oldOff.Status=off.Status
		if err:=r.Update(ctx,oldOff);err!=nil{
			return ctrl.Result{},err
		}
	}
	return ctrl.Result{}, nil
}

func (r *OfflineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&colocationv1.Offline{}).
		Complete(r)
}


func Key(off *colocationv1.Offline)string{
	return types.NamespacedName{Name:off.Name,Namespace:off.Namespace}.String()
}