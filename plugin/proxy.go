package plugin

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// SetupProxies mounts declarative reverse-proxy mini-apps from a spec like
//
//	PLUGIN_PROXY=open-panneau=http://127.0.0.1:3020,irc-chat=http://127.0.0.1:3021
//
// Requests to /<id> and /<id>/* are forwarded to the upstream with the full
// path preserved — which is exactly how the Node app serves its plugins, so
// an unmigrated Node mini-app keeps working behind the Go core unchanged.
func SetupProxies(mux *http.ServeMux, spec string) {
	for _, entry := range splitCSV(spec) {
		id, upstream, ok := strings.Cut(entry, "=")
		id = strings.TrimSpace(strings.TrimPrefix(id, "/"))
		upstream = strings.TrimSpace(upstream)
		if !ok || id == "" || upstream == "" {
			log.Printf("[proxy] skipping malformed PLUGIN_PROXY entry %q", entry)
			continue
		}
		target, err := url.Parse(upstream)
		if err != nil || target.Host == "" {
			log.Printf("[proxy] skipping %s: bad upstream %q", id, upstream)
			continue
		}
		proxy := &httputil.ReverseProxy{
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(target) // keeps the original path
				pr.SetXForwarded()
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("[proxy:%s] upstream error: %v", id, err)
				http.Error(w, "Le service "+id+" est momentanément indisponible.", http.StatusBadGateway)
			},
		}
		mux.Handle("/"+id, proxy)
		mux.Handle("/"+id+"/", proxy)
		log.Printf("[proxy] /%s → %s", id, upstream)
	}
}
