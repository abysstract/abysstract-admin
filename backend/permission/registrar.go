package permission

// Authenticated is the permission sentinel for procedures that only require a
// logged-in user.
const Authenticated = "@authenticated"

// Entry is a protected Connect procedure permission declaration.
// Public procedures must be registered with RegisterPublic; an empty
// Permission is not a public marker.
type Entry struct {
	Procedure  string
	Permission string
}

// Registrar collects generated and public procedure permissions during
// application bootstrap.
type Registrar interface {
	RegisterGenerated(...Entry) error
	RegisterPublic(...string) error
}
