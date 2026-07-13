package plugin

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
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

// SHA256Hex hashes a secret the way the Node plugins do
// (crypto.createHash('sha256').update(s).digest('hex')), so credentials
// created by either app verify in the other.
func SHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// CheckBasicAuth reports whether the request carries HTTP Basic credentials
// matching user/pass (constant-time). False when user or pass is empty.
func CheckBasicAuth(r *http.Request, user, pass string) bool {
	if user == "" || pass == "" {
		return false
	}
	u, p, ok := r.BasicAuth()
	return ok &&
		subtle.ConstantTimeCompare([]byte(u), []byte(user)) == 1 &&
		subtle.ConstantTimeCompare([]byte(p), []byte(pass)) == 1
}

// ParseDate accepts the date formats browsers actually send — full RFC3339,
// datetime-local ("2006-01-02T15:04"), and bare date inputs ("2006-01-02") —
// matching the leniency of JavaScript's new Date() that the Node app relied on.
func ParseDate(s string) (time.Time, bool) {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
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
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(data)
	}
}
