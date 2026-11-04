package repo

import (
	"time"

	"github.com/abysstract/abysstract-admin/backend/resource"
)

func NormalizeTimestamp(t time.Time) time.Time { return resource.NormalizeTimestamp(t) }
