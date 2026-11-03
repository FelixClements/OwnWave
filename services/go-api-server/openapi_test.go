package main

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"ownwave/api/internal/analytics"
)

type openAPIDoc struct {
	Paths      map[string]map[string]any `yaml:"paths"`
	Components map[string]map[string]any `yaml:"components"`
}

func loadSpec(t *testing.T) (openAPIDoc, any) {
	t.Helper()
	var doc openAPIDoc
	require.NoError(t, yaml.Unmarshal(openAPISpec, &doc))
	var raw any
	require.NoError(t, yaml.Unmarshal(openAPISpec, &raw))
	return doc, raw
}

var httpMethods = map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true}

// TestOpenAPIListsEveryRoute fails when a route is added or removed
// without updating openapi.yaml.
func TestOpenAPIListsEveryRoute(t *testing.T) {
	doc, _ := loadSpec(t)
	var documented []string
	for path, ops := range doc.Paths {
		for method := range ops {
			if httpMethods[method] {
				documented = append(documented, strings.ToUpper(method)+" "+path)
			}
		}
	}

	h := newTestHandler(analytics.NewStub())
	routes := map[string]bool{}
	err := chi.Walk(newRouter(h, "").(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.TrimSuffix(route, "/")
		// /api/admin/metrics is mounted with Handle (every method); only GET is meaningful.
		if route == "/api/admin/metrics" && method != http.MethodGet {
			return nil
		}
		if httpMethods[strings.ToLower(method)] {
			routes[method+" "+route] = true
		}
		return nil
	})
	require.NoError(t, err)
	var served []string
	for r := range routes {
		served = append(served, r)
	}
	sort.Strings(documented)
	sort.Strings(served)
	require.Equal(t, served, documented)
}

// TestOpenAPIRefsResolve fails on a $ref to a missing component.
func TestOpenAPIRefsResolve(t *testing.T) {
	doc, raw := loadSpec(t)
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, val := range x {
				if k == "$ref" {
					ref, _ := val.(string)
					parts := strings.Split(strings.TrimPrefix(ref, "#/components/"), "/")
					require.Len(t, parts, 2, ref)
					_, ok := doc.Components[parts[0]][parts[1]]
					require.True(t, ok, "unresolved %s", ref)
				}
				walk(val)
			}
		case []any:
			for _, val := range x {
				walk(val)
			}
		}
	}
	walk(raw)
}

func TestOpenAPIIsServed(t *testing.T) {
	rec := httptestGet(t, newRouter(newTestHandler(analytics.NewStub()), ""), "/api/openapi.yaml")
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, strings.HasPrefix(rec.Body.String(), "openapi: 3.1.0"))
}

func httptestGet(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// TestOpenAPIOperationIDsAreUnique: generated clients name methods after them.
func TestOpenAPIOperationIDsAreUnique(t *testing.T) {
	doc, _ := loadSpec(t)
	seen := map[string]string{}
	for path, ops := range doc.Paths {
		for method, op := range ops {
			if !httpMethods[method] {
				continue
			}
			id, _ := op.(map[string]any)["operationId"].(string)
			require.NotEmpty(t, id, "%s %s has no operationId", method, path)
			require.Empty(t, seen[id], "operationId %s used twice", id)
			seen[id] = method + " " + path
		}
	}
}
