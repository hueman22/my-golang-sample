package http

import (
	"net/http"

	domcategory "example.com/my-golang-sample/app/internal/domain/category"
	domorder "example.com/my-golang-sample/app/internal/domain/order"
	domproduct "example.com/my-golang-sample/app/internal/domain/product"
	domuser "example.com/my-golang-sample/app/internal/domain/user"
	domrole "example.com/my-golang-sample/app/internal/domain/userrole"
	useruc "example.com/my-golang-sample/app/internal/usecase/user"
	userroleuc "example.com/my-golang-sample/app/internal/usecase/userrole"
)

type createRoleRequest struct {
	Code        string `json:"code" validate:"required"`
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
}

type updateRoleRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (a *API) handleListUserRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := a.roleSvc.List(r.Context(), domrole.ListFilter{})
	if err != nil {
		handleDomainError(w, err)
		return
	}

	resp := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		resp = append(resp, mapRole(role))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": resp})
}

func (a *API) handleCreateUserRole(w http.ResponseWriter, r *http.Request) {
	var req createRoleRequest
	if err := a.decodeAndValidate(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	role, err := a.roleSvc.Create(r.Context(), userroleuc.CreateInput{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapRole(role))
}

func (a *API) handleUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	var req updateRoleRequest
	if err := a.decodeAndValidate(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	role, err := a.roleSvc.Update(r.Context(), userroleuc.UpdateInput{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapRole(role))
}

func (a *API) handleDeleteUserRole(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	if err := a.roleSvc.Delete(r.Context(), id); err != nil {
		handleDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type createUserRequest struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
	RoleCode string `json:"role_code" validate:"required"`
}

type updateUserRequest struct {
	Name     *string `json:"name"`
	Email    *string `json:"email" validate:"omitempty,email"`
	Password *string `json:"password"`
	RoleCode *string `json:"role_code"`
}

func (a *API) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.userSvc.ListUsers(r.Context(), domuser.ListUsersFilter{})
	if err != nil {
		handleDomainError(w, err)
		return
	}
	resp := make([]map[string]any, 0, len(users))
	for _, u := range users {
		resp = append(resp, mapUser(u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": resp})
}

func (a *API) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	executor := getAuthUser(r.Context())
	if executor == nil {
		respondError(w, http.StatusUnauthorized, errUnauthenticated)
		return
	}

	var req createUserRequest
	if err := a.decodeAndValidate(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	role, err := domuser.ParseRoleCode(req.RoleCode)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	user, err := a.userSvc.CreateUser(r.Context(), useruc.CreateUserInput{
		ExecutorRole: executor.RoleCode,
		Name:         req.Name,
		Email:        req.Email,
		Password:     req.Password,
		RoleCode:     role,
	})
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapUser(user))
}

func (a *API) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	u, err := a.userSvc.GetUser(r.Context(), id)
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapUser(u))
}

func (a *API) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	executor := getAuthUser(r.Context())
	if executor == nil {
		respondError(w, http.StatusUnauthorized, errUnauthenticated)
		return
	}

	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	var req updateUserRequest
	if err := a.decodeAndValidate(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	var roleCode *domuser.RoleCode
	if req.RoleCode != nil {
		role, err := domuser.ParseRoleCode(*req.RoleCode)
		if err != nil {
			handleDomainError(w, err)
			return
		}
		roleCode = &role
	}

	user, err := a.userSvc.UpdateUser(r.Context(), useruc.UpdateUserInput{
		ExecutorRole: executor.RoleCode,
		ID:           id,
		Name:         req.Name,
		Email:        req.Email,
		Password:     req.Password,
		RoleCode:     roleCode,
	})
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapUser(user))
}

func (a *API) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	if err := a.userSvc.DeleteUser(r.Context(), id); err != nil {
		handleDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type categoryRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
}

func (a *API) handleListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := a.categorySvc.List(r.Context(), domcategory.ListFilter{})
	if err != nil {
		handleDomainError(w, err)
		return
	}
	resp := make([]map[string]any, 0, len(categories))
	for _, c := range categories {
		resp = append(resp, mapCategory(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": resp})
}

func (a *API) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	var req categoryRequest
	if err := a.decodeAndValidate(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	category, err := a.categorySvc.Create(r.Context(), &domcategory.Category{
		Name:        req.Name,
		Description: req.Description,
		IsActive:    req.IsActive,
	})
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapCategory(category))
}

func (a *API) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	var req categoryRequest
	if err := a.decodeAndValidate(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	category, err := a.categorySvc.Update(r.Context(), &domcategory.Category{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		IsActive:    req.IsActive,
	})
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapCategory(category))
}

func (a *API) handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	if err := a.categorySvc.Delete(r.Context(), id); err != nil {
		handleDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type productRequest struct {
	Name        string  `json:"name" validate:"required"`
	Description string  `json:"description"`
	Price       float64 `json:"price" validate:"required,gt=0"`
	Stock       int64   `json:"stock" validate:"required,gte=0"`
	CategoryID  int64   `json:"category_id" validate:"required,gt=0"`
	IsActive    bool    `json:"is_active"`
}

func (a *API) handleCreateProduct(w http.ResponseWriter, r *http.Request) {
	var req productRequest
	if err := a.decodeAndValidate(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	product, err := a.productSvc.Create(r.Context(), &domproduct.Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		CategoryID:  req.CategoryID,
		IsActive:    req.IsActive,
	})
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapProduct(product))
}

func (a *API) handleUpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	var req productRequest
	if err := a.decodeAndValidate(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	product, err := a.productSvc.Update(r.Context(), &domproduct.Product{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		CategoryID:  req.CategoryID,
		IsActive:    req.IsActive,
	})
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapProduct(product))
}

func (a *API) handleDeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	if err := a.productSvc.Delete(r.Context(), id); err != nil {
		handleDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type updateOrderStatusRequest struct {
	Status string `json:"status" validate:"required"`
}

func (a *API) handleListOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := a.orderSvc.List(r.Context())
	if err != nil {
		handleDomainError(w, err)
		return
	}
	resp := make([]map[string]any, 0, len(orders))
	for _, o := range orders {
		resp = append(resp, mapOrder(o))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": resp})
}

func (a *API) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	order, err := a.orderSvc.GetByID(r.Context(), id)
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapOrder(order))
}

func (a *API) handleUpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	var req updateOrderStatusRequest
	if err := a.decodeAndValidate(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	status := domorder.Status(req.Status)
	order, err := a.orderSvc.UpdateStatus(r.Context(), id, status)
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapOrder(order))
}

