package http

import (
	"net/http"

	authuc "example.com/my-golang-sample/app/internal/usecase/auth"
)

type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := a.decodeAndValidate(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	result, err := a.authSvc.Login(r.Context(), authuc.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token": result.Token,
		"user":  mapUser(result.User),
	})
}

