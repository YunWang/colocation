package controllers

import (
	"context"
	"github.com/YunWang/colocation/api/v1"
	"github.com/YunWang/colocation/pkg/utils"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime"

)

type PodReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme        *runtime.Scheme
	lastSeenPhase map[string]corev1.PodPhase
}

func (pr *PodReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := pr.Log.WithValues("pod", req.NamespacedName)

	//1.get pod
	pod := &corev1.Pod{}
	if err := pr.Get(ctx, req.NamespacedName, pod);err!=nil{
		if errors.IsNotFound(err){
			log.V(1).Info("Pod was deleted")
			return ctrl.Result{},nil
		}
		return ctrl.Result{}, err
	}

	//2.get Offline
	var offlineName string
	owners:=pod.ObjectMeta.OwnerReferences
	for _,owner := range owners {
		if owner.APIVersion== OfflineAPIVersion && owner.Kind == OfflineKind {
			offlineName = owner.Name
			break
		}
	}

	//we only care about offline pod
	if offlineName == ""{
		return ctrl.Result{},nil
	}
	//3.get offline
	off:=&v1.Offline{}
	if err:=pr.Get(ctx,types.NamespacedName{Name:offlineName,Namespace:pod.Namespace},off);err!=nil{
		//TODO should delete this pod and return nil
		return ctrl.Result{},err
	}
	//4.get lastPhase
	lastPhase,exist:=pr.lastSeenPhase[req.NamespacedName.String()]

	//pod creation
	if !exist{
		pr.lastSeenPhase[req.NamespacedName.String()]=pod.Status.Phase
		//add finalizer
		if _,exist:=utils.ContainsString(pod.Finalizers,OfflineFinalizer);!exist{
			pod.Finalizers=append(pod.Finalizers, OfflineFinalizer)
		}

		//sync
		oldPod:=&corev1.Pod{}
		err := pr.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, oldPod)
		if err != nil {
			return ctrl.Result{},err
		}
		if !reflect.DeepEqual(oldPod.Finalizers, pod.Finalizers) {
			oldPod.Finalizers = pod.Finalizers
			if err = pr.Update(ctx, oldPod); err != nil {
				pr.Log.Info("Update Pod failed")
				return ctrl.Result{},err
			}
		}
		return ctrl.Result{},nil
	}

	//we only care aboud two events. First, deleteTimestamp changed.Second
	//pod.status.phase changed
	//5.deleteTimestamp changed, that means pod would be deleted
	if !pod.DeletionTimestamp.IsZero() {
		//deletePodEvent
		if index,ok:=utils.ContainsString(pod.ObjectMeta.Finalizers,OfflineFinalizer);ok{
			//update gang status
			if err:=pr.syncOfflineStatus(ctx,off,lastPhase,"");err!=nil{
				return ctrl.Result{},err
			}
			//delete pod.NamespacedName from lastSeenPhase
			delete(pr.lastSeenPhase,req.NamespacedName.String())
			//delete finalizer from pod.Finalizers
			if err:=pr.removePodFinalizer(ctx,pod,index);err!=nil{
				return ctrl.Result{},err
			}
		}else{
			//??????
		}
		return ctrl.Result{},nil
	}
	//6.pod status.phase changed,that means pod's phase changed
	if err:=pr.syncOfflineStatus(ctx,off,lastPhase,pod.Status.Phase);err!=nil{
		return ctrl.Result{},err
	}
	//7.update lastSeenPhase
	pr.lastSeenPhase[req.NamespacedName.String()]=pod.Status.Phase
	return ctrl.Result{}, nil
}

func (pr *PodReconciler) syncOfflineStatus(ctx context.Context, offline *v1.Offline,lastPhase,currentPhase corev1.PodPhase) error {
	pr.calculateOfflineStatus(offline,lastPhase,currentPhase)

	oldOff:=&v1.Offline{}
	err := pr.Get(ctx, types.NamespacedName{Name: offline.Name, Namespace: offline.Namespace}, oldOff)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(oldOff.Status, offline.Status) {
		oldOff.Status = offline.Status
		if err = pr.Update(ctx, oldOff); err != nil {
			pr.Log.Info("Update Offline failed")
			return err
		}
	}
	return nil
}

func (pr *PodReconciler)calculateOfflineStatus(offline *v1.Offline,lastPhase,currentPhase corev1.PodPhase){
	if lastPhase ==currentPhase{
		return
	}
	//update lastPhase
	if lastPhase!=""{
		pr.calculatePodNumber(offline,lastPhase,-1)
	}
	//update currentPhase
	if currentPhase!=""{
		pr.calculatePodNumber(offline,currentPhase,1)
	}
}

func (pr *PodReconciler)calculatePodNumber(offline *v1.Offline,phase corev1.PodPhase,offset int32){
	switch phase {
	case corev1.PodRunning:
		offline.Status.PodRunning+=offset
	case corev1.PodPending:
		offline.Status.PodPending+=offset
	case corev1.PodFailed:
		offline.Status.PodFailed+=offset
	case corev1.PodUnknown:
		offline.Status.PodUnknown+=offset
	case corev1.PodSucceeded:
		offline.Status.PodSucceeded+=offset
	}
}

func (pr *PodReconciler) removePodFinalizer(ctx context.Context,pod *corev1.Pod,finalizerIndex int32)error{
	pod.Finalizers=append(pod.Finalizers[:finalizerIndex],pod.Finalizers[finalizerIndex+1:]...)

	oldPod:=&corev1.Pod{}
	if err:=pr.Get(ctx,types.NamespacedName{Namespace:pod.Namespace,Name:pod.Name},oldPod);err!=nil{
		return err
	}
	if !reflect.DeepEqual(oldPod.Finalizers,pod.Finalizers){
		oldPod.Finalizers=pod.Finalizers
		if err:=pr.Update(ctx,oldPod);err!=nil{
			return err
		}
	}
	return nil
}


func NewPodController(client client.Client, log logr.Logger, scheme *runtime.Scheme) *PodReconciler {
	return &PodReconciler{
		Client:        client,
		Log:           log,
		Scheme:        scheme,
		lastSeenPhase: make(map[string]corev1.PodPhase),
	}
}