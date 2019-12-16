package utils

import (
	"github.com/YunWang/colocation/api/v1"
	"k8s.io/apimachinery/pkg/types"
)

func LessFn(offline1, offline2 interface{}) bool {
	o1 := offline1.(v1.Offline)
	o2 := offline2.(v1.Offline)
	prio1 := o1.Spec.Level
	prio2 := o2.Spec.Level
	return (prio1 > prio2) || (prio1 == prio2 && o1.ObjectMeta.CreationTimestamp.Before(&o2.ObjectMeta.CreationTimestamp))
}

func KeyFn(obj interface{}) (string, error) {
	offline:=obj.(*v1.Offline)
	return types.NamespacedName{Namespace:offline.Namespace,Name:offline.Name}.String(), nil
}

func GetOfflineQueueName(off *v1.Offline)string{
	queueName:=off.Spec.Queue
	if queueName ==""{
		queueName="default"
	}
	return queueName
}

func ContainsString(strs []string,key string)(int32,bool){
	for index,elem:=range strs{
		if elem==key {
			return int32(index),true
		}
	}
	return -1,false
}