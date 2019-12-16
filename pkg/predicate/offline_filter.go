package predicate

import "sigs.k8s.io/controller-runtime/pkg/event"

type OfflineFilter struct {
	//predicate.Funcs
}

func (p *OfflineFilter) Create(createEvent event.CreateEvent) bool {
	if createEvent.Meta.GetNamespace()=="default" {
		return true
	}
	return false
}
func (p *OfflineFilter) Update(updateEvent event.UpdateEvent) bool {
	if updateEvent.MetaNew.GetNamespace()=="default" {
		return true
	}
	return false
}
func (p *OfflineFilter) Delete(deleteEvent event.DeleteEvent) bool {
	//only status change should be passed and deletetimestamp changed should be passed
	//the former indicated pod's status changed
	//the latter indicated pod had been deleted
	if deleteEvent.Meta.GetNamespace()=="default" {
		return true
	}
	return false
}
func (p *OfflineFilter) Generic(genericEvent event.GenericEvent) bool {
	if genericEvent.Meta.GetNamespace()=="default" {
		return true
	}
	return false
}