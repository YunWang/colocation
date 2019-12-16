package cache

import (
	"fmt"
	"github.com/YunWang/colocation/api/v1"
	"k8s.io/klog"
)

type Cache struct {
	queues map[string]*Queue
	unschedulableQ map[string]struct{}
}

func(c *Cache) Add(name string)error{
	if _,exist:=c.queues[name];exist {
		klog.V(0).Info("Queue %v has existed!",name)
		return nil
	}
	c.queues[name]=NewQueue()
	return nil
}

func(c *Cache) Delete(name string)error{
	if _,exist := c.queues[name];!exist {
		klog.V(0).Info("Queue %v isn't exist",name)
		return nil
	}
	delete(c.queues, name)
	return nil
}

func(c *Cache) Get(name string) *Queue{
	if _,exist := c.queues[name];!exist{
		c.queues[name]=NewQueue()
		return c.queues[name]
	}
	return c.queues[name]
}

func(c *Cache) AddToUnSchedulableQ(off *v1.Offline) error{
	key:=c.key(off)
	if _,exist:=c.unschedulableQ[key];exist{
		klog.V(0).Info("Offline has existed, you cann't add again!")
		return nil
	}
	c.unschedulableQ[key]= struct{}{}
	return nil
}

func(c *Cache) DeleteFromUnSchedulableQ(off *v1.Offline)error{
	key:=c.key(off)
	if _,exist:=c.unschedulableQ[key];!exist{
		klog.V(0).Info("Offline isn't exist!")
		return nil
	}
	delete(c.unschedulableQ, key)
	return nil
}

func(c *Cache) key(off *v1.Offline)string{
	return fmt.Sprintf("%s/%s", off.Namespace, off.Name)
}

func New()*Cache{
	c:= &Cache{
		queues:make(map[string]*Queue),
		unschedulableQ: make(map[string]struct{}),
	}
	c.queues["default"]=NewQueue()
	return c
}