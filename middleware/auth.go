package middleware

import (
	"github.com/olusolaa/go-backend/pkg/account"
	"net/http"
)

var authUserId int64

func BasicAuth(findUserByEmail func(string) (*account.Account, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
				w.WriteHeader(403)
				w.Write([]byte("403 Unauthorized\n"))
				return
			}
			acc, err := findUserByEmail(user)
			if err != nil {
				w.WriteHeader(403)
				w.Write([]byte("403 Unauthorized\n"))
				return
			}
			if acc.AuthId != pass {
				w.WriteHeader(403)
				w.Write([]byte("403 Unauthorized\n"))
				return
			}
			authUserId = acc.ID
			next.ServeHTTP(w, r)
		})
	}
}

func GetAuthUserId() int64 {
	return authUserId
}
