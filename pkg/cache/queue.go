package cache

import (
	"github.com/YunWang/colocation/api/v1"
	"github.com/YunWang/colocation/pkg/utils"
	"k8s.io/client-go/tools/cache"
	"sync"
)

type Queue struct {
	lock         sync.RWMutex
	schedulingQ  *cache.Heap
	current		 string
	name 		 string
}

func (q *Queue) Pop() (*v1.Offline,error){
	off,err:=q.schedulingQ.Pop()
	if err!=nil{
		return nil,err
	}
	return off.(*v1.Offline),nil
}
func (q *Queue) Len()int32{
	return int32(len(q.schedulingQ.ListKeys()))
}

func(q *Queue) GetName()string{
	return q.name
}

func (q *Queue) AddSchedulingQ(offline *v1.Offline) error{
	return q.schedulingQ.Add(offline)
}

func (q *Queue) Get(key string) (*v1.Offline,bool){
	obj,exist,_:=q.schedulingQ.GetByKey(key)
	if !exist {
		return nil,false
	}
	return obj.(*v1.Offline),true
}

func (q *Queue) List() {

}

func (q *Queue) Peek() {

}

func(q *Queue) Scheduling()string{
	return q.current
}
func (q *Queue) Delete(offline *v1.Offline) error {
	return q.schedulingQ.Delete(offline)
}

func NewQueue() *Queue {
	return &Queue{
		schedulingQ:  cache.NewHeap(utils.KeyFn, utils.LessFn),
	}
}
