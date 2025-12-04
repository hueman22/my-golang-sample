package http

import (
	"net/http"
)

// GET /api/v1/admin/user-roles/{id}
func (a *API) handleGetUserRole(w http.ResponseWriter, r *http.Request) {
	// Bảo vệ: phải login & có user trong context
	user := getAuthUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, errUnauthenticated)
		return
	}

	// Lấy param id từ URL
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	// Gọi service để lấy role theo ID
	role, err := a.roleSvc.GetByID(r.Context(), id)
	if err != nil {
		// Map domain error -> HTTP (404, 500, ...)
		handleDomainError(w, err)
		return
	}

	// Trả JSON ra client
	writeJSON(w, http.StatusOK, mapRole(role))
}
