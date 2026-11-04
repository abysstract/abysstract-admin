package service

import (
	"testing"
)

// Feature: open-source-readiness, Property 3: 分页参数归一化
func TestNormalizePageStrict(t *testing.T) {
	tests := []struct {
		name     string
		pageNum  int
		pageSize int
		wantNum  int
		wantSize int
		wantErr  bool
	}{
		{name: "page_size 负数返回错误", pageNum: 1, pageSize: -1, wantErr: true},
		{name: "page_size 为 0 取默认 10", pageNum: 1, pageSize: 0, wantNum: 1, wantSize: 10},
		{name: "page_size 在 1..100 原样返回", pageNum: 2, pageSize: 50, wantNum: 2, wantSize: 50},
		{name: "page_size 超过 100 截断为 100", pageNum: 1, pageSize: 200, wantNum: 1, wantSize: 100},
		{name: "page_num 小于 1 归一化为 1", pageNum: 0, pageSize: 20, wantNum: 1, wantSize: 20},
		{name: "page_num 负数归一化为 1", pageNum: -5, pageSize: 20, wantNum: 1, wantSize: 20},
		{name: "page_size 为 100 边界原样返回", pageNum: 3, pageSize: 100, wantNum: 3, wantSize: 100},
		{name: "page_size 为 1 边界原样返回", pageNum: 1, pageSize: 1, wantNum: 1, wantSize: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNum, gotSize, err := normalizePageStrict(tt.pageNum, tt.pageSize)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if gotNum != tt.wantNum {
				t.Errorf("pageNum = %d, want %d", gotNum, tt.wantNum)
			}
			if gotSize != tt.wantSize {
				t.Errorf("pageSize = %d, want %d", gotSize, tt.wantSize)
			}
		})
	}
}
