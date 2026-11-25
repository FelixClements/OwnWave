package analytics

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Access names the auth guard a proxied route sits behind. The router applies
// the guard; the proxy only forwards.
type Access int

const (
	AdminOnly   Access = iota // auth.RequireAdmin
	AnyUser                   // auth.RequireUser
	SetupAccess               // the setup guard: setup token or admin
)

// Route maps a public path under /api to the Python path it forwards to.
// Path parameters keep their names in both paths and must be UUIDs.
type Route struct {
	Method string
	Public string
	Python string
	Access Access
}

// Routes is every analytics route Go passes through unchanged. openapi.yaml
// marks each one with x-python-path: a Go test checks the marks against this
// table, and a Python test checks the response schemas against FastAPI's.
var Routes = []Route{
	{http.MethodPost, "/admin/scan", "/scan", AdminOnly},
	{http.MethodGet, "/admin/scan/{id}", "/jobs/{id}", AdminOnly},
	{http.MethodGet, "/admin/scans", "/jobs", AdminOnly},
	{http.MethodPost, "/admin/rebuild-analysis", "/rebuild-analysis", AdminOnly},
	{http.MethodPost, "/admin/rebuild-genres", "/rebuild-genres", AdminOnly},
	{http.MethodPost, "/admin/outside-genres/refetch", "/outside-genres/refetch", AdminOnly},
	{http.MethodGet, "/admin/outside-sources", "/outside-sources", AdminOnly},
	{http.MethodPost, "/admin/outside-sources/settings", "/outside-sources/settings", AdminOnly},
	{http.MethodPost, "/admin/outside-sources/test", "/outside-sources/test", AdminOnly},
	{http.MethodGet, "/admin/genres/report", "/genres/report", AdminOnly},
	{http.MethodGet, "/genres", "/genres", AnyUser},
	{http.MethodGet, "/tracks/{id}/genres", "/tracks/{id}/genres", AnyUser},
	{http.MethodGet, "/setup/summary", "/setup/summary", SetupAccess},
}

// RoutesFor returns the routes behind one guard.
func RoutesFor(access Access) []Route {
	var out []Route
	for _, rt := range Routes {
		if rt.Access == access {
			out = append(out, rt)
		}
	}
	return out
}

// Proxy forwards Routes to the Python analytics service. It sends only the
// internal token and JSON headers; the caller's cookies and Authorization
// header stay in Go.
type Proxy struct {
	base      *url.URL
	baseErr   error
	secret    string
	transport http.RoundTripper
	onError   func(http.ResponseWriter, *http.Request, error)
}

// NewProxy reads the internal token from ANALYTICS_API_SECRET. onError
// answers when Python can't be reached.
func NewProxy(baseURL string, onError func(http.ResponseWriter, *http.Request, error)) *Proxy {
	return NewProxyWithSecret(baseURL, os.Getenv("ANALYTICS_API_SECRET"), onError)
}

func NewProxyWithSecret(baseURL, secret string, onError func(http.ResponseWriter, *http.Request, error)) *Proxy {
	base, err := url.Parse(strings.TrimRight(baseURL, "/"))
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = requestTimeout
	return &Proxy{base: base, baseErr: err, secret: secret, transport: transport, onError: onError}
}

var pathParam = regexp.MustCompile(`\{([^}]+)\}`)

// Handler forwards one route. It answers 400 when a path parameter is not a
// UUID, as the hand-written handlers did.
func (p *Proxy) Handler(rt Route) http.Handler {
	var params []string
	for _, m := range pathParam.FindAllStringSubmatch(rt.Public, -1) {
		params = append(params, m[1])
	}
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			path := rt.Python
			for _, name := range params {
				path = strings.ReplaceAll(path, "{"+name+"}", chi.URLParam(pr.In, name))
			}
			pr.Out.URL = &url.URL{
				Scheme:   p.base.Scheme,
				Host:     p.base.Host,
				Path:     p.base.Path + path,
				RawQuery: pr.In.URL.RawQuery,
			}
			pr.Out.Host = ""
			pr.Out.Header = http.Header{"Accept": {"application/json"}}
			// FastAPI parses a body only when it is labelled JSON.
			if pr.In.ContentLength != 0 {
				pr.Out.Header.Set("Content-Type", "application/json")
			}
			if p.secret != "" {
				pr.Out.Header.Set("X-Internal-Token", p.secret)
			}
		},
		Transport:    p.transport,
		ErrorHandler: p.onError,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p.baseErr != nil {
			p.onError(w, r, p.baseErr)
			return
		}
		for _, name := range params {
			if _, err := uuid.Parse(chi.URLParam(r, name)); err != nil {
				http.Error(w, "invalid "+name, http.StatusBadRequest)
				return
			}
		}
		rp.ServeHTTP(w, r)
	})
}
