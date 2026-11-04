package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/xuri/excelize/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

// BatchDelete 上限（需求 9.8）：>1000 拒绝且不删除任何记录。
const maxBatchDelete = 1000

// Excel 导出上限（需求 9.5）：>100,000 拒绝。
const maxExportRows = 100_000

var (
	errBatchDeleteEmpty      = errors.New("批量删除的 id 列表不能为空")
	errBatchDeleteTooMany    = fmt.Errorf("批量删除记录数超过上限 %d", maxBatchDelete)
	errBatchDeleteUnknownRes = errors.New("未知的资源类型")

	errImportUnknownRes   = errors.New("未知的导入资源类型")
	errImportEmptyContent = errors.New("导入文件内容不能为空")
	errImportParseFailure = errors.New("电子表格文件解析失败")

	errExportUnknownRes = errors.New("未知的导出资源类型")
	errExportTooMany    = fmt.Errorf("导出行数超过上限 %d", maxExportRows)
)

// BatchDeleter 资源级批量删除器抽象。
// 返回实际删除条数；由 service 注册表按 resource 派发。
type BatchDeleter interface {
	Delete(ctx context.Context, ids []int64) (int64, error)
}

// userExporter 导出用户列表（需求 9.5）。
// 行：ID / 账号 / 昵称 / 邮箱 / 手机 / 状态 / 创建时间。
type userExporter struct{ repo *repo.Repo }

func (e *userExporter) Header() []string {
	return []string{"ID", "账号", "昵称", "邮箱", "手机", "状态", "创建时间"}
}

func (e *userExporter) Rows(ctx context.Context, keyword string, status int16) ([][]string, error) {
	// 用 ListUsersForExport 而非 ListUsers：后者会经 normalizePage 把 pageSize 截断到 100，
	// 导致 >100 行的导出静默丢失数据。
	users, err := e.repo.ListUsersForExport(ctx, keyword, status, maxExportRows)
	if err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(users))
	for _, u := range users {
		st := "停用"
		if u.Status == 1 {
			st = "正常"
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", u.ID),
			u.Username,
			u.Nickname,
			u.Email,
			u.Phone,
			st,
			u.CreateTime.Format("2006-01-02 15:04:05"),
		})
	}
	return rows, nil
}

// userImporter 导入用户（需求 9.6）。
// 列顺序：[账号, 密码, 昵称, 邮箱, 手机]；账号与密码必填，其余可空。
// 逐行校验：缺失必填 / 复杂度不达标 / 重复 username 的行写入 failures，
// 有效行调用 repo.CreateUser 持久化（不带角色，默认值由 DB 填充）。
type userImporter struct{ repo *repo.Repo }

func (i *userImporter) Header() []string {
	return []string{"账号", "密码", "昵称", "邮箱", "手机"}
}

func (i *userImporter) Import(ctx context.Context, rows [][]string) ([]importRowResult, error) {
	out := make([]importRowResult, len(rows))
	for idx, r := range rows {
		// 列数不足时补齐，避免越界。
		for len(r) < 5 {
			r = append(r, "")
		}
		username, password := r[0], r[1]
		nickname, email, phone := r[2], r[3], r[4]

		if username == "" {
			out[idx] = importRowResult{ok: false, reason: "账号为空"}
			continue
		}
		if password == "" {
			out[idx] = importRowResult{ok: false, reason: "密码为空"}
			continue
		}
		if err := validatePasswordPolicy(password); err != nil {
			out[idx] = importRowResult{ok: false, reason: err.Error()}
			continue
		}

		u := &model.SysUser{
			Username: username,
			Nickname: nickname,
			Email:    email,
			Phone:    phone,
			Status:   1,
		}
		if err := i.repo.CreateUser(ctx, u, password, nil); err != nil {
			out[idx] = importRowResult{ok: false, reason: err.Error()}
			continue
		}
		out[idx] = importRowResult{ok: true}
	}
	return out, nil
}

// ExcelExporter 资源级 Excel 导出器抽象（需求 9.5）。
// 实现返回行集合（不含表头），由 service 包装成 xlsx。
type ExcelExporter interface {
	Header() []string
	Rows(ctx context.Context, keyword string, status int16) ([][]string, error)
}

// ExcelImporter 资源级 Excel 导入器抽象（需求 9.6）。
// Import 对每一行返回 (ok, reason)；ok=false 即视为失败行，reason 写入失败明细。
type ExcelImporter interface {
	Header() []string
	Import(ctx context.Context, rows [][]string) (results []importRowResult, err error)
}

type importRowResult struct {
	ok     bool
	reason string
}

// PermChecker 校验当前调用者是否拥有指定权限标识（如 "system:user:remove"）。
// 无权限时返回 connect.CodePermissionDenied 错误；其余错误原样返回。
// 由 main.go 注入：基于 interceptor.ClaimsFromContext + repo.PermsOf 实现。
//
// 通用 RPC 在拦截器层仅声明 AuthSentinel（仅需登录），具体资源权限
// （resource:list / :add / :remove）在 service 层按 resource 派发时校验，
// 因为 resource 标识来自请求消息，拦截器拿不到。
type PermChecker func(ctx context.Context, perm string) error

// PermLoader 返回指定用户的权限标识集合（来自 repo.PermsOf）。
type PermLoader func(ctx context.Context, userID int64) ([]string, error)

// NewPermChecker 构造一个基于 interceptor.ClaimsFromContext + PermLoader 的 PermChecker。
// 无 claims → Unauthenticated；有 claims 但无权限 → PermissionDenied。
// admin 通配 "*:*:*" 自动通过（由 interceptor.HasPerm 处理）。
func NewPermChecker(loader PermLoader) PermChecker {
	return func(ctx context.Context, perm string) error {
		claims, ok := interceptor.ClaimsFromContext(ctx)
		if !ok {
			return connect.NewError(connect.CodeUnauthenticated, "未获取到登录信息")
		}
		perms, err := loader(ctx, claims.UserID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
		}
		if !interceptor.HasPerm(perms, perm) {
			return connect.Errorf(
				connect.CodePermissionDenied,
				"缺少权限: %s", perm,
			)
		}
		return nil
	}
}

// CommonExportService 实现 adminv1connect.CommonExportServiceHandler。
// 资源派发通过 deleters / exporters / importers 注册表完成。
type CommonExportService struct {
	deleters  map[string]BatchDeleter
	exporters map[string]ExcelExporter
	importers map[string]ExcelImporter
	perm      PermChecker
}

// NewCommonExportService 创建空注册表的实例；具体资源由 With* 方法挂载。
// perm 可为 nil（测试场景默认放行）；生产构造由 main.go 注入非空实现。
func NewCommonExportService(r *repo.Repo, perm PermChecker) *CommonExportService {
	svc := &CommonExportService{
		deleters:  make(map[string]BatchDeleter),
		exporters: make(map[string]ExcelExporter),
		importers: make(map[string]ExcelImporter),
		perm:      perm,
	}
	if r != nil {
		svc.exporters["system:user"] = &userExporter{repo: r}
		svc.importers["system:user"] = &userImporter{repo: r}
	}
	return svc
}

// checkPerm 调用 perm 校验；perm 为 nil 时放行（仅测试场景）。
func (s *CommonExportService) checkPerm(ctx context.Context, perm string) error {
	if s.perm == nil {
		return nil
	}
	return s.perm(ctx, perm)
}

// RegisterDeleter 注册资源批量删除器（扩展点）。
func (s *CommonExportService) RegisterDeleter(resource string, d BatchDeleter) {
	s.deleters[resource] = d
}

// RegisterExporter 注册资源导出器。
func (s *CommonExportService) RegisterExporter(resource string, e ExcelExporter) {
	s.exporters[resource] = e
}

// RegisterImporter 注册资源导入器。
func (s *CommonExportService) RegisterImporter(resource string, im ExcelImporter) {
	s.importers[resource] = im
}

// BatchDelete 批量删除（需求 9.7/9.8）。
// 上限校验先于派发：>1000 直接拒绝且不调用任何 deleter（保证「不删除任何记录」）。
// 资源权限（需求 5.x）：派发前按 resource 校验 :remove 权限。
func (s *CommonExportService) BatchDelete(
	ctx context.Context, req *adminv1.BatchDeleteRequest,
) (*adminv1.BatchDeleteResponse, error) {
	msg := req
	// 鉴权先于参数校验：避免向无权限调用者泄漏资源是否存在。
	if err := s.checkPerm(ctx, msg.Resource+":remove"); err != nil {
		return nil, err
	}
	if len(msg.Ids) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errBatchDeleteEmpty.Error()).WithCause(errBatchDeleteEmpty)
	}
	if len(msg.Ids) > maxBatchDelete {
		return nil, connect.NewError(connect.CodeInvalidArgument, errBatchDeleteTooMany.Error()).WithCause(errBatchDeleteTooMany)
	}
	deleter, ok := s.deleters[msg.Resource]
	if !ok {
		cause := fmt.Errorf("%w: %s", errBatchDeleteUnknownRes, msg.Resource)
		return nil, connect.NewError(connect.CodeInvalidArgument, cause.Error()).WithCause(cause)
	}
	deleted, err := deleter.Delete(ctx, msg.Ids)
	if err != nil {
		return nil, toConnectError(err)
	}
	return &adminv1.BatchDeleteResponse{Deleted: int32(deleted)}, nil
}

// ExportExcel 导出当前查询结果为 xlsx（需求 9.5）。
// 行数上限 100,000，越界返回 InvalidArgument。
// 资源权限：派发前按 resource 校验 :list 权限（导出本质是读列表）。
func (s *CommonExportService) ExportExcel(
	ctx context.Context, req *adminv1.ExportExcelRequest,
) (*adminv1.ExportExcelResponse, error) {
	msg := req
	if err := s.checkPerm(ctx, msg.Resource+":list"); err != nil {
		return nil, err
	}
	exporter, ok := s.exporters[msg.Resource]
	if !ok {
		cause := fmt.Errorf("%w: %s", errExportUnknownRes, msg.Resource)
		return nil, connect.NewError(connect.CodeInvalidArgument, cause.Error()).WithCause(cause)
	}
	var status int16
	if msg.Status != nil {
		status = int16(*msg.Status)
	}
	rows, err := exporter.Rows(ctx, msg.Keyword, status)
	if err != nil {
		return nil, toConnectError(err)
	}
	if len(rows) > maxExportRows {
		return nil, connect.NewError(connect.CodeInvalidArgument, errExportTooMany.Error()).WithCause(errExportTooMany)
	}
	content, err := buildXLSX(exporter.Header(), rows)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	filename := fmt.Sprintf("%s-%s.xlsx", msg.Resource, time.Now().Format("20060102"))
	return &adminv1.ExportExcelResponse{
		Content:  content,
		Filename: filename,
		RowCount: int32(len(rows)),
	}, nil
}

// ImportExcel 逐行校验并写入有效行（需求 9.6）。
// 行号约定：数据行从 1 开始（不含表头）。
// 属性 15 保证：每条输入行要么进入成功集合，要么进入失败明细，且二者不相交。
// 资源权限：派发前按 resource 校验 :add 权限（导入本质是写入）。
func (s *CommonExportService) ImportExcel(
	ctx context.Context, req *adminv1.ImportExcelRequest,
) (*adminv1.ImportExcelResponse, error) {
	msg := req
	if err := s.checkPerm(ctx, msg.Resource+":add"); err != nil {
		return nil, err
	}
	if len(msg.Content) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errImportEmptyContent.Error()).WithCause(errImportEmptyContent)
	}
	importer, ok := s.importers[msg.Resource]
	if !ok {
		cause := fmt.Errorf("%w: %s", errImportUnknownRes, msg.Resource)
		return nil, connect.NewError(connect.CodeInvalidArgument, cause.Error()).WithCause(cause)
	}
	rows, err := parseXLSXRows(msg.Content)
	if err != nil {
		cause := fmt.Errorf("%w: %v", errImportParseFailure, err)
		return nil, connect.NewError(connect.CodeInvalidArgument, cause.Error()).WithCause(cause)
	}
	if len(rows) == 0 {
		return &adminv1.ImportExcelResponse{
			Total:        0,
			SuccessCount: 0,
			Failures:     []*adminv1.ImportFailure{},
		}, nil
	}
	// 第 1 行视为表头，数据行从第 2 行（rows[1:]）开始。
	// rows 长度为 1（只有表头）时 dataRows 为空切片，不进入 importer。
	dataRows := rows[1:]
	results, err := importer.Import(ctx, dataRows)
	if err != nil {
		return nil, toConnectError(err)
	}
	resp := &adminv1.ImportExcelResponse{
		Total:        int32(len(dataRows)),
		SuccessCount: 0,
		Failures:     []*adminv1.ImportFailure{},
	}
	for i, r := range results {
		// 行号从 1 开始（与 Excel 习惯一致：第 2 行是第一条数据）。
		rowNum := int32(i + 1)
		if r.ok {
			resp.SuccessCount++
		} else {
			resp.Failures = append(resp.Failures, &adminv1.ImportFailure{
				Row:    rowNum,
				Reason: r.reason,
			})
		}
	}
	return resp, nil
}

// buildXLSX 用 excelize 写表头 + 行，输出二进制。
func buildXLSX(header []string, rows [][]string) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"
	// 表头
	for i, h := range header {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, err
		}
	}
	// 数据行
	for r, row := range rows {
		for c, v := range row {
			cell, err := excelize.CoordinatesToCellName(c+1, r+2)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return nil, err
			}
		}
	}
	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// parseXLSXRows 解析 xlsx 二进制，返回按行划分的字符串矩阵。
func parseXLSXRows(content []byte) ([][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("电子表格文件无工作表")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = [][]string{}
	}
	return rows, nil
}
