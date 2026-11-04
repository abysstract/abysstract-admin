package module

import "net/http"

// Access is the declared HTTP access policy for one route.
type Access uint8

const (
	AccessUnspecified Access = iota
	AccessPublic
	AccessAuthenticated
	AccessPermission
)

// HTTPRoute is the static declaration of one ordinary HTTP route.
type HTTPRoute struct {
	Name       string
	Method     string
	Path       string
	Subtree    bool
	Access     Access
	Permission string
	MaxBytes   int64
}

// HTTPHandler binds a created handler to a declared route Name.
type HTTPHandler struct {
	Name    string
	Handler http.Handler
}
