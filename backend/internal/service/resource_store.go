package service

import (
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func newResourceStore(base *repo.Repo) (*resource.Store, error) {
	if base == nil {
		return resource.NewStore(nil)
	}
	return resource.NewStore(base.DB())
}
