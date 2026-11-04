package resource

// 分页公共规范：统一默认值与上限，避免各列表接口重复实现，
// 也防止恶意/误操作的超大分页拖垮数据库。
const (
	DefaultPageSize = 10
	MaxPageSize     = 100
)

// NormalizePage 归一化分页参数：页码至少为 1，页大小落在 [1, MaxPageSize]。
func NormalizePage(pageNum, pageSize int) (int, int) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return pageNum, pageSize
}

// Page carries pagination parameters for repository lists. Generated List
// implementations apply the package defaults and maximum.
type Page struct {
	Num  int
	Size int
}
