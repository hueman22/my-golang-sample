package http

import (
	"context"
	"errors"
	"net/http"
	"strings"

	domuser "example.com/my-golang-sample/app/internal/domain/user"
)

var (
	ctxUserKey         = struct{}{}
	errUnauthenticated = errors.New("unauthenticated")
	errForbidden       = errors.New("forbidden")
)

type authUser struct {
	UserID   int64
	RoleCode domuser.RoleCode
	Email    string
	Name     string
}

func (a *API) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			respondError(w, http.StatusUnauthorized, errUnauthenticated)
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		claims, err := a.tokenSvc.ParseToken(token)
		if err != nil {
			respondError(w, http.StatusUnauthorized, errUnauthenticated)
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserKey, &authUser{
			UserID:   claims.UserID,
			RoleCode: claims.RoleCode,
			Email:    claims.Email,
			Name:     claims.Name,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *API) requireRoles(roles ...domuser.RoleCode) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := getAuthUser(r.Context())
			if user == nil {
				respondError(w, http.StatusUnauthorized, errUnauthenticated)
				return
			}
			for _, role := range roles {
				if user.RoleCode == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			respondError(w, http.StatusForbidden, errForbidden)
		})
	}
}

func getAuthUser(ctx context.Context) *authUser {
	val := ctx.Value(ctxUserKey)
	if user, ok := val.(*authUser); ok {
		return user
	}
	return nil
}
