package module

import (
	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ConnectService declares a service descriptor for startup validation and
// permission registration. Created instances register their handlers on the
// application server through Register.
type ConnectService struct {
	Descriptor protoreflect.ServiceDescriptor
	Register   func(*connect.Server)
}
