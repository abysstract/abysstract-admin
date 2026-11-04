package repo

import "github.com/abysstract/abysstract-admin/backend/resource"

type Page = resource.Page

const DefaultPageSize = resource.DefaultPageSize
const MaxPageSize = resource.MaxPageSize

func normalizePage(num, size int) (int, int) { return resource.NormalizePage(num, size) }
