package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Small HTTP helpers exposed to plugins so every mini-app answers with the
// same JSON conventions as the core.

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func WriteErr(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

func DecodeBody(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v)
}

// DBCtx returns a 5s-bounded context for a request's database work.
func DBCtx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 5*time.Second)
}

// ServeWebFile serves one file from the plugin's overlaid web FS — the
// standard way a plugin exposes its (static) view.
func ServeWebFile(ctx *Context, name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := ctx.Web.ReadFile(name)
		if err != nil {
			http.Error(w, "view unavailable", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}
}
