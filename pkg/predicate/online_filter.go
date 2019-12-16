package predicate

import "sigs.k8s.io/controller-runtime/pkg/event"

type NamespacePredicate struct {
	//predicate.Funcs
}

func (p *NamespacePredicate) Create(createEvent event.CreateEvent) bool {

	if createEvent.Meta.GetNamespace()=="default" {
		return true
	}
	return false
}
func (p *NamespacePredicate) Update(updateEvent event.UpdateEvent) bool {
	if updateEvent.MetaNew.GetNamespace()=="default" {
		return true
	}
	return false
}
func (p *NamespacePredicate) Delete(deleteEvent event.DeleteEvent) bool {
	//only status change should be passed and deletetimestamp changed should be passed
	//the former indicated pod's status changed
	//the latter indicated pod had been deleted
	if deleteEvent.Meta.GetNamespace()=="default" {
		return true
	}
	return false
}
func (p *NamespacePredicate) Generic(genericEvent event.GenericEvent) bool {
	if genericEvent.Meta.GetNamespace()=="default" {
		return true
	}
	return false
}