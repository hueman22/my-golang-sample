package http

import (
	"encoding/json"
	"net/http"

	dom "example.com/my-golang-sample/app/internal/domain/user"
	uc "example.com/my-golang-sample/app/internal/usecase/user"
)

type UserHandler struct {
	svc *uc.Service
}

func NewUserHandler(svc *uc.Service) *UserHandler {
	return &UserHandler{svc: svc}
}

type UserAPIRequest struct {
	Action       string `json:"action"` // create|get|update|delete
	ID           int64  `json:"id,omitempty"`
	Name         string `json:"name,omitempty"`
	Email        string `json:"email,omitempty"`
	RoleCode     string `json:"role_code,omitempty"`
	ExecutorRole string `json:"executor_role,omitempty"`
}

type UserResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	RoleCode string `json:"role_code"`
}

func (h *UserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UserAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	execRole, err := dom.ParseRoleCode(req.ExecutorRole)
	if err != nil {
		http.Error(w, "invalid executor_role", http.StatusUnprocessableEntity)
		return
	}

	switch req.Action {
	case "create":
		role, err := dom.ParseRoleCode(req.RoleCode)
		if err != nil {
			http.Error(w, "invalid role_code", http.StatusUnprocessableEntity)
			return
		}

		u, err := h.svc.CreateUser(r.Context(), uc.CreateUserInput{
			ExecutorRole: execRole,
			Name:         req.Name,
			Email:        req.Email,
			RoleCode:     role,
		})
		if err != nil {
			handleError(w, err)
			return
		}
		writeJSON(w, toUserResponse(u), http.StatusCreated)

	case "get":
		u, err := h.svc.GetUser(r.Context(), req.ID)
		if err != nil {
			handleError(w, err)
			return
		}
		writeJSON(w, toUserResponse(u), http.StatusOK)

	case "update":
		var rolePtr *dom.RoleCode
		if req.RoleCode != "" {
			role, err := dom.ParseRoleCode(req.RoleCode)
			if err != nil {
				http.Error(w, "invalid role_code", http.StatusUnprocessableEntity)
				return
			}
			rolePtr = &role
		}

		var namePtr, emailPtr *string
		if req.Name != "" {
			namePtr = &req.Name
		}
		if req.Email != "" {
			emailPtr = &req.Email
		}

		u, err := h.svc.UpdateUser(r.Context(), uc.UpdateUserInput{
			ExecutorRole: execRole,
			ID:           req.ID,
			Name:         namePtr,
			Email:        emailPtr,
			RoleCode:     rolePtr,
		})
		if err != nil {
			handleError(w, err)
			return
		}
		writeJSON(w, toUserResponse(u), http.StatusOK)

	case "delete":
		if err := h.svc.DeleteUser(r.Context(), req.ID); err != nil {
			handleError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "unsupported action", http.StatusBadRequest)
	}
}

func toUserResponse(u *dom.User) UserResponse {
	return UserResponse{
		ID:       u.ID,
		Name:     u.Name,
		Email:    u.Email,
		RoleCode: string(u.RoleCode),
	}
}

func writeJSON(w http.ResponseWriter, v any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func handleError(w http.ResponseWriter, err error) {
	switch err {
	case dom.ErrCannotAssignRole:
		http.Error(w, err.Error(), http.StatusUnprocessableEntity) // 422
	case dom.ErrUserNotFound:
		http.Error(w, err.Error(), http.StatusNotFound)
	case dom.ErrInvalidRoleCode:
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	default:
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
