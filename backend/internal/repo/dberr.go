package repo

import "github.com/abysstract/abysstract-admin/backend/resource"

var ErrDuplicate = resource.ErrDuplicate
var ErrCycle = resource.ErrCycle

func wrapDBErr(err error) error { return resource.NormalizeDBError(err) }
