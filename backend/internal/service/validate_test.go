package service

import (
	"testing"

	"connectrpc.com/connect/v2"
)

func TestValidatorPhone(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "optional empty value", value: "", want: true},
		{name: "mainland mobile", value: "13800000000", want: true},
		{name: "mainland mobile 19 prefix", value: "19912345678", want: true},
		{name: "too short", value: "1380000000", want: false},
		{name: "invalid prefix", value: "12800000000", want: false},
		{name: "international prefix", value: "+8613800000000", want: false},
		{name: "contains whitespace", value: "138 0000 0000", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := newValidator().phone("手机号", tt.value).err()
			if tt.want && err != nil {
				t.Fatalf("phone(%q) = %v, want nil", tt.value, err)
			}
			if !tt.want {
				if connect.CodeOf(err) != connect.CodeInvalidArgument {
					t.Fatalf("phone(%q) code = %v, want InvalidArgument; err=%v", tt.value, connect.CodeOf(err), err)
				}
				if err.Error() != "invalid_argument: 手机号格式不正确" {
					t.Fatalf("phone(%q) error = %v, want 手机号格式不正确", tt.value, err)
				}
			}
		})
	}
}
