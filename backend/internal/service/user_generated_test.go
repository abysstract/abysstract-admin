package service

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

func TestNewGeneratedUserServiceRejectsNilBaseRepo(t *testing.T) {
	if svc, err := NewGeneratedUserService(nil); err == nil || svc != nil {
		t.Fatalf("NewGeneratedUserService(nil) = svc=%v err=%v, want nil service and error", svc, err)
	}
}

func TestUserCustomCreateUsesGeneratedSchemaLengthValidation(t *testing.T) {
	svc := NewUserService(nil)
	long := func(n int) string {
		return strings.Repeat("x", n)
	}

	_, err := svc.CreateUser(context.Background(), &adminv1.CreateUserRequest{
		Username: "schema-validation",
		Email:    long(129),
		Password: "GoodPass1",
		RoleIds:  []int64{1},
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "email") {
		t.Fatalf("long create email error = %v, want InvalidArgument mentioning email", err)
	}

	_, err = svc.CreateUser(context.Background(), &adminv1.CreateUserRequest{
		Username: "schema-validation",
		Password: "GoodPass1",
		Remark:   long(501),
		RoleIds:  []int64{1},
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "remark") {
		t.Fatalf("long create remark error = %v, want InvalidArgument mentioning remark", err)
	}
}

func TestUserCustomUpdateUsesGeneratedSchemaLengthValidation(t *testing.T) {
	svc := NewUserService(nil)
	long := strings.Repeat("x", 501)
	remark := long
	_, err := svc.UpdateUser(context.Background(), &adminv1.UpdateUserRequest{
		Id:     2,
		Remark: &remark,
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "remark") {
		t.Fatalf("long update remark error = %v, want InvalidArgument mentioning remark", err)
	}
}
