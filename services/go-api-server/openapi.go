package main

import (
	_ "embed"
	"net/http"
)

// openAPISpec documents every route; openapi_test.go keeps it in sync
// with the router.
//
//go:embed openapi.yaml
var openAPISpec []byte

func serveOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(openAPISpec)
}
