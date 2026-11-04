package module

import "fmt"

// protocolFields retains retired fields so their wire numbers cannot be reused.
type protocolFields map[string]protocolField
type protocolField struct {
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Managed  bool   `json:"managed"`
	Entity   int    `json:"entity"`
	Create   int    `json:"create,omitempty"`
	Retired  bool   `json:"retired,omitempty"`
}

func assignProtocol(view *crudView, previous protocolFields) (protocolFields, error) {
	next := protocolFields{}
	maxEntity, maxCreate := 1, 0
	for name, field := range previous {
		field.Retired = true
		next[name] = field
		if field.Entity > maxEntity {
			maxEntity = field.Entity
		}
		if field.Create > maxCreate {
			maxCreate = field.Create
		}
	}
	for i := range view.Fields {
		f := &view.Fields[i]
		old, exists := previous[f.Column]
		if exists {
			if old.Retired {
				return nil, fmt.Errorf("field %s is retired; choose a new API field name", f.Column)
			}
			if old.Type != f.ProtoType || old.Nullable != f.Nullable || old.Managed != f.Managed {
				return nil, fmt.Errorf("field %s changes its protocol type, presence or ownership; introduce a versioned contract", f.Column)
			}
		} else {
			maxEntity++
			old = protocolField{Type: f.ProtoType, Nullable: f.Nullable, Managed: f.Managed, Entity: maxEntity}
			if !f.Managed {
				maxCreate++
				old.Create = maxCreate
			}
		}
		old.Retired = false
		next[f.Column] = old
		f.EntityNumber, f.CreateNumber = old.Entity, old.Create
	}
	return next, nil
}
