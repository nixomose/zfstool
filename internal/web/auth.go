package web

import (
	"net"
	"net/http"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func isLoopbackRequest(r *http.Request) bool {
	// RemoteAddr is ip:port
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// authMiddleware: loopback skips auth; non-loopback requires Basic auth.
// If env ZFSTOOL_WEB_USER and ZFSTOOL_WEB_PASSWORD are set, verify plain password.
// If ZFSTOOL_WEB_BCRYPT_HASH is set (with ZFSTOOL_WEB_USER), verify bcrypt.
// Build tag `pam` can extend with PAM (see auth_pam.go).
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isLoopbackRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		if pamAuth != nil && s.PAMService != "" {
			user, pass, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="zfstool"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if err := pamAuth(s.PAMService, user, pass); err != nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		u := os.Getenv("ZFSTOOL_WEB_USER")
		p := os.Getenv("ZFSTOOL_WEB_PASSWORD")
		hash := os.Getenv("ZFSTOOL_WEB_BCRYPT_HASH")
		if u == "" || (p == "" && hash == "") {
			http.Error(w, "remote access disabled: set ZFSTOOL_WEB_USER and ZFSTOOL_WEB_PASSWORD (or ZFSTOOL_WEB_BCRYPT_HASH), or build with -tags pam and libpam", http.StatusForbidden)
			return
		}
		ru, rp, ok := r.BasicAuth()
		if !ok || ru != u {
			w.Header().Set("WWW-Authenticate", `Basic realm="zfstool"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if hash != "" {
			if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(rp)); err != nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		} else {
			if rp != p {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// pamAuth is set by auth_pam.go when built with tag pam.
var pamAuth func(service, user, pass string) error
