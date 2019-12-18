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
	"fmt"
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
	log := r.Log.WithValues("offline", req.NamespacedName)

	off:=&colocationv1.Offline{}
	if err:=r.Client.Get(ctx,req.NamespacedName,off);err!=nil{
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	queueName:=utils.GetOfflineQueueName(off)
	queue:=r.Cache.Get(queueName)

	//create
	if off.ObjectMeta.CreationTimestamp.IsZero() {
		log.V(0).Info("Offline{"+off.Name+"} Created!")
		//createTimestamp
		off.ObjectMeta.CreationTimestamp=v1.Now()
		//add to scheduling queue

		if err:=queue.AddSchedulingQ(off);err!=nil{
			return ctrl.Result{},err
		}
		//finalizer for delete
		off.Finalizers=append(off.ObjectMeta.Finalizers, OfflineFinalizer)
		//offline pending
		off.Status.Phase=colocationv1.OfflinePendingPhase

		//update to cluster
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
		return ctrl.Result{},nil
	}
	//Get all pod owned by this offline
	podList:=&v12.PodList{}
	if err:=r.Client.List(ctx,podList,client.InNamespace(req.Namespace),client.MatchingLabelsSelector{labels.SelectorFromSet(off.Spec.Selector.MatchLabels)});err!=nil{
		return ctrl.Result{},err
	}

	//delete
	if !off.DeletionTimestamp.IsZero() {
		log.V(0).Info("Offline{"+off.Name+"} Deleted!")
		//set deletetimestamp
		for _,pod := range podList.Items {
			pod.ObjectMeta.DeletionTimestamp=&v1.Time{Time:time.Now()}
			go r.updatePod(ctx,&pod)
		}
		//delete offline from queue
		//queueName:=utils.GetOfflineQueueName(off)
		//queue:=r.Cache.Get(queueName)
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



	//handle queue according to offline phase
	if off.Status.Phase==colocationv1.OfflineFailedPhase {
		//delete whole offline to release resource besides failed pod,because user may want to check failed pod
		for _,pod := range podList.Items {
			if pod.Status.Phase==v12.PodFailed {
				continue
			}
			pod.ObjectMeta.DeletionTimestamp=&v1.Time{Time:time.Now()}
			go r.updatePod(ctx,&pod)
		}
		_=r.Cache.AddToUnSchedulableQ(off)
		current:=queue.GetCurrent()
		if Key(current) == Key(off) {
			if err:=queue.UpdateCurrent();err!=nil{
				return ctrl.Result{},nil
			}
			current=queue.GetCurrent()
			if current!=nil {
				_=r.startOffline(ctx,current)
			}else{
				_=r.Cache.Delete(queueName)
			}
		}else{
			_,exist:=queue.Get(Key(off))
			if exist {
				_=queue.Delete(off)
			}
		}
	}else if off.Status.Phase==colocationv1.OfflineRunningPhase{
		currentSchedulingOffline:=queue.GetCurrent()
		if Key(currentSchedulingOffline)== Key(off){
			//scheduling next offline
			_=queue.UpdateCurrent()
			next:=queue.GetCurrent()
			if next==nil{
				_=r.Cache.Delete(queueName)
			}else{
				//start offline
				_=r.startOffline(ctx,next)
			}
		}else{
			target,exist:=queue.Get(Key(off))
			if exist {
				if err:=queue.Delete(target);err!=nil{
					return ctrl.Result{},err
				}
			}
		}
	}else if off.Status.Phase == colocationv1.OfflinePendingPhase{
		//add to schedulingQ
		_=queue.AddSchedulingQ(off)
		if queue.GetCurrent()==nil {
			//start offline
			_=queue.UpdateCurrent()
			current:=queue.GetCurrent()
			if current == nil{
				_=r.Cache.Delete(queueName)
			}else{
				_=r.startOffline(ctx,queue.GetCurrent())
			}
		}
	}else if off.Status.Phase==colocationv1.OfflineSchedulingPhase {
		_,exist:=queue.Get(Key(off))
		if !exist {
			_=queue.AddSchedulingQ(off)
		}
		exist=r.Cache.IsExistInUnSchedulableQ(off)
		if exist {
			_=r.Cache.DeleteFromUnSchedulableQ(off)
		}
	}else if off.Status.Phase==colocationv1.OfflineSucceededPhase{
		_,exist:=queue.Get(Key(off))
		if !exist {
			_=queue.AddSchedulingQ(off)
		}
	}

	//update to cluster
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

func(r *OfflineReconciler) updatePod(ctx context.Context,pod *v12.Pod)error{
	if err:=r.Client.Update(ctx,pod,&client.UpdateOptions{});err!=nil {
		return err
	}
	return nil
}

func (r *OfflineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&colocationv1.Offline{}).
		Complete(r)
}

func(r *OfflineReconciler) startOffline(ctx context.Context,off *colocationv1.Offline)error{
	for _,podTemplateSpec := range off.Spec.Tasks {
		pod:=&v12.Pod{
			ObjectMeta:v1.ObjectMeta{
				Labels:getPodsLabelSet(podTemplateSpec),
				Annotations:getPodsAnnotationSet(podTemplateSpec),
				Finalizers:getPodsFinalizers(podTemplateSpec),
				GenerateName:fmt.Sprintf("%s-", off.Name),
			},
		}
		flag:=true
		pod.OwnerReferences = append(pod.OwnerReferences, v1.OwnerReference{
			APIVersion:off.APIVersion,
			Kind:off.Kind,
			Name:off.Name,
			UID:off.UID,
			Controller:&flag,
		})
		pod.Spec=*podTemplateSpec.Spec.DeepCopy()
		go func(pod *v12.Pod) {
			if err:=r.Client.Create(ctx,pod,&client.CreateOptions{});err!=nil{
				return
			}
		}(pod)
	}
	return nil
}

func getPodsLabelSet(template *v12.PodTemplateSpec)labels.Set{
	desiredLabels := make(labels.Set)
	for k, v := range template.Labels {
		desiredLabels[k] = v
	}
	return desiredLabels
}

func getPodsAnnotationSet(template *v12.PodTemplateSpec) labels.Set {
	desiredAnnotations := make(labels.Set)
	for k, v := range template.Annotations {
		desiredAnnotations[k] = v
	}
	return desiredAnnotations
}
func getPodsFinalizers(template *v12.PodTemplateSpec) []string {
	desiredFinalizers := make([]string, len(template.Finalizers))
	copy(desiredFinalizers, template.Finalizers)
	return desiredFinalizers
}

func Key(off *colocationv1.Offline)string{
	return types.NamespacedName{Name:off.Name,Namespace:off.Namespace}.String()
}