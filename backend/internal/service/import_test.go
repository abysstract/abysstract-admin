package service

import (
	"context"
	"testing"
	"testing/quick"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

// fakeImporter 接受「第 1 列为 ok 标记」的数据行：
// 行首字符串等于 "ok" 视为成功；否则视为失败并返回行首作为 reason。
type fakeImporter struct{}

func (fakeImporter) Header() []string { return []string{"flag", "payload"} }

func (fakeImporter) Import(_ context.Context, rows [][]string) ([]importRowResult, error) {
	out := make([]importRowResult, len(rows))
	for i, r := range rows {
		if len(r) == 0 {
			out[i] = importRowResult{ok: false, reason: "空行"}
			continue
		}
		if r[0] == "ok" {
			out[i] = importRowResult{ok: true}
		} else {
			out[i] = importRowResult{ok: false, reason: "行首非 ok: " + r[0]}
		}
	}
	return out, nil
}

// 属性 15：Excel 导入逐行结果划分。
// 对任意输入行集合：成功集合 ∪ 失败集合 = 输入集合；成功集合 ∩ 失败集合 = ∅；
// 失败集合的每条都附带行号且行号在 [1, len] 范围内；成功行数 + 失败行数 = 总行数。
func TestPropertyImportExcelPartition(t *testing.T) {
	svc := NewCommonExportService(nil, nil)
	svc.RegisterImporter("test:res", fakeImporter{})

	// 构造 xlsx：第 1 行表头，后续为数据行。
	mkxlsx := func(dataRows [][]string) []byte {
		f := newSimpleXLSX(t, []string{"flag", "payload"}, dataRows)
		return f
	}

	check := func(rows [][]string) bool {
		content := mkxlsx(rows)
		resp, err := svc.ImportExcel(context.Background(),
			&adminv1.ImportExcelRequest{Resource: "test:res", Content: content})
		if err != nil {
			t.Errorf("unexpected err: %v", err)
			return false
		}
		r := resp
		// 总行数 = 输入数据行数（不含表头）
		if int(r.Total) != len(rows) {
			t.Errorf("total mismatch: got %d want %d", r.Total, len(rows))
			return false
		}
		// 成功 + 失败 = 总数
		if int(r.SuccessCount)+len(r.Failures) != int(r.Total) {
			t.Errorf("partition violation: success=%d + failures=%d != total=%d",
				r.SuccessCount, len(r.Failures), r.Total)
			return false
		}
		// 行号唯一且在范围内
		seen := make(map[int32]bool, len(r.Failures))
		for _, f := range r.Failures {
			if f.Row < 1 || f.Row > int32(len(rows)) {
				t.Errorf("row number out of range: %d", f.Row)
				return false
			}
			if seen[f.Row] {
				t.Errorf("duplicate failure row: %d", f.Row)
				return false
			}
			seen[f.Row] = true
			if f.Reason == "" {
				t.Errorf("failure reason empty for row %d", f.Row)
				return false
			}
			// 行首非 "ok" 的行必出现在 failures 中
			if rows[f.Row-1][0] == "ok" {
				t.Errorf("row %d marked as failure but starts with ok", f.Row)
				return false
			}
		}
		// 成功行的行号未在 failures 中
		for i, r := range rows {
			if len(r) > 0 && r[0] == "ok" {
				if seen[int32(i+1)] {
					t.Errorf("row %d marked both ok and failure", i+1)
					return false
				}
			}
		}
		return true
	}

	// 枚举若干典型输入。
	// 注：excelize GetRows 会丢弃 xlsx 末尾的完全空行（Excel 格式限制），
	// 因此属性 15 的「输入行」语义为「Excel 解析返回的行集合」，测试用例不依赖尾随空行。
	cases := [][][]string{
		{},
		{{"ok", "a"}},
		{{"bad", "a"}},
		{{"ok", "a"}, {"bad", "b"}, {"ok", "c"}, {"ugly", "d"}},
		{{""}, {"ok", "x"}, {"bad", "y"}},
	}
	for i, c := range cases {
		if !check(c) {
			t.Fatalf("case #%d failed", i)
		}
	}

	// 随机属性测试（200 次）：随机生成 0–20 行，每行随机 ok/bad。
	if err := quick.Check(func(seed int) bool {
		n := seed % 21
		if n < 0 {
			n = -n
		}
		rows := make([][]string, n)
		for i := 0; i < n; i++ {
			flag := "bad"
			if (seed+i)%2 == 0 {
				flag = "ok"
			}
			rows[i] = []string{flag, "x"}
		}
		return check(rows)
	}, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatalf("quick check failed: %v", err)
	}
}

// 导入失败 / 边界：未知资源、空内容。
func TestImportExcelFailures(t *testing.T) {
	svc := NewCommonExportService(nil, nil)
	svc.RegisterImporter("test:res", fakeImporter{})

	// 未知资源
	_, err := svc.ImportExcel(context.Background(),
		&adminv1.ImportExcelRequest{Resource: "nope:none", Content: []byte("x")})
	if err == nil {
		t.Fatalf("expected error for unknown resource")
	}

	// 空内容
	_, err = svc.ImportExcel(context.Background(),
		&adminv1.ImportExcelRequest{Resource: "test:res", Content: nil})
	if err == nil {
		t.Fatalf("expected error for empty content")
	}
}

// newSimpleXLSX 构造 xlsx 二进制：header + data rows。
func newSimpleXLSX(t *testing.T, header []string, rows [][]string) []byte {
	t.Helper()
	content, err := buildXLSX(header, rows)
	if err != nil {
		t.Fatalf("build xlsx: %v", err)
	}
	return content
}
