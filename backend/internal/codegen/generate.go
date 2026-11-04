// Package codegen owns the application's offline generation entry.
package codegen

//go:generate node ../../../frontend/tools/generate-proto.mjs
//go:generate go run ../../cmd/adminresource-gen
