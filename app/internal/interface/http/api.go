package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"

	domcart "example.com/my-golang-sample/app/internal/domain/cart"
	domcategory "example.com/my-golang-sample/app/internal/domain/category"
	domorder "example.com/my-golang-sample/app/internal/domain/order"
	domproduct "example.com/my-golang-sample/app/internal/domain/product"
	domuser "example.com/my-golang-sample/app/internal/domain/user"
	domrole "example.com/my-golang-sample/app/internal/domain/userrole"
	authuc "example.com/my-golang-sample/app/internal/usecase/auth"
	cartuc "example.com/my-golang-sample/app/internal/usecase/cart"
	categoryuc "example.com/my-golang-sample/app/internal/usecase/category"
	orderuc "example.com/my-golang-sample/app/internal/usecase/order"
	productuc "example.com/my-golang-sample/app/internal/usecase/product"
	useruc "example.com/my-golang-sample/app/internal/usecase/user"
	userroleuc "example.com/my-golang-sample/app/internal/usecase/userrole"
)

type API struct {
	authSvc     *authuc.Service
	userSvc     *useruc.Service
	roleSvc     *userroleuc.Service
	categorySvc *categoryuc.Service
	productSvc  *productuc.Service
	cartSvc     *cartuc.Service
	orderSvc    *orderuc.Service
	validator   *validator.Validate
	tokenSvc    authuc.TokenService
}

type Dependencies struct {
	AuthService     *authuc.Service
	UserService     *useruc.Service
	UserRoleService *userroleuc.Service
	CategoryService *categoryuc.Service
	ProductService  *productuc.Service
	CartService     *cartuc.Service
	OrderService    *orderuc.Service
	TokenService    authuc.TokenService
}

func NewAPI(deps Dependencies) *API {
	validate := validator.New()
	return &API{
		authSvc:     deps.AuthService,
		userSvc:     deps.UserService,
		roleSvc:     deps.UserRoleService,
		categorySvc: deps.CategoryService,
		productSvc:  deps.ProductService,
		cartSvc:     deps.CartService,
		orderSvc:    deps.OrderService,
		tokenSvc:    deps.TokenService,
		validator:   validate,
	}
}

func (a *API) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.AllowContentType("application/json", "text/plain"))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", a.handleLogin)
		r.Get("/products", a.handleListProducts)
		r.Get("/products/{id}", a.handleGetProduct)

		r.Group(func(pr chi.Router) {
			pr.Use(a.authMiddleware)
			pr.Get("/me/cart", a.handleGetCart)
			pr.Post("/me/cart/items", a.handleAddCartItem)
			pr.Post("/me/checkout", a.handleCheckout)
		})

		r.Group(func(ar chi.Router) {
			ar.Use(a.authMiddleware)
			ar.Use(a.requireRoles(domuser.RoleCodeAdmin, domuser.RoleCodeSuperAdmin))

			ar.Route("/admin", func(admin chi.Router) {
				admin.Route("/user-roles", func(rr chi.Router) {
					rr.Get("/", a.handleListUserRoles)
					rr.Post("/", a.handleCreateUserRole)
					rr.Get("/{id}", a.handleGetUserRole)
					rr.Put("/{id}", a.handleUpdateUserRole)
					rr.Delete("/{id}", a.handleDeleteUserRole)
				})

				admin.Route("/users", func(rr chi.Router) {
					rr.Get("/", a.handleListUsers)
					rr.Post("/", a.handleCreateUser)
					rr.Get("/{id}", a.handleGetUser)
					rr.Put("/{id}", a.handleUpdateUser)
					rr.Delete("/{id}", a.handleDeleteUser)
				})

				admin.Route("/categories", func(rr chi.Router) {
					rr.Get("/", a.handleListCategories)
					rr.Post("/", a.handleCreateCategory)
					rr.Get("/{id}", a.handleGetCategory)
					rr.Put("/{id}", a.handleUpdateCategory)
					rr.Delete("/{id}", a.handleDeleteCategory)
				})

				admin.Route("/products", func(rr chi.Router) {
					rr.Get("/", a.handleListProductsAdmin)
					rr.Post("/", a.handleCreateProduct)
					rr.Put("/{id}", a.handleUpdateProduct)
					rr.Delete("/{id}", a.handleDeleteProduct)
				})

				admin.Route("/orders", func(rr chi.Router) {
					rr.Get("/", a.handleListOrders)
					rr.Get("/{id}", a.handleGetOrder)
					rr.Patch("/{id}", a.handleUpdateOrderStatus)
				})
			})
		})
	})

	return r
}

func (a *API) decodeAndValidate(r *http.Request, dst any) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return err
	}
	return a.validator.Struct(dst)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type errorResponse struct {
	Error   string `json:"error"`
	Details any    `json:"details,omitempty"`
}

func respondError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, errorResponse{Error: err.Error()})
}

func parseIDParam(r *http.Request, key string) (int64, error) {
	idStr := chi.URLParam(r, key)
	return strconv.ParseInt(idStr, 10, 64)
}

func mapUser(u *domuser.User) map[string]any {
	return map[string]any{
		"id":        u.ID,
		"name":      u.Name,
		"email":     u.Email,
		"role_code": u.RoleCode,
	}
}

func mapRole(role *domrole.UserRole) map[string]any {
	return map[string]any{
		"id":          role.ID,
		"code":        role.Code,
		"name":        role.Name,
		"description": role.Description,
		"is_system":   role.IsSystem,
	}
}

func mapCategory(c *domcategory.Category) map[string]any {
	return map[string]any{
		"id":          c.ID,
		"name":        c.Name,
		"slug":        c.Slug,
		"description": c.Description,
		"is_active":   c.IsActive,
	}
}

func mapProduct(p *domproduct.Product) map[string]any {
	return map[string]any{
		"id":          p.ID,
		"name":        p.Name,
		"description": p.Description,
		"price":       p.Price,
		"stock":       p.Stock,
		"category_id": p.CategoryID,
		"is_active":   p.IsActive,
	}
}

func mapCart(cart *domcart.Cart) map[string]any {
	items := make([]map[string]any, 0, len(cart.Items))
	for _, item := range cart.Items {
		items = append(items, map[string]any{
			"product_id": item.ProductID,
			"quantity":   item.Quantity,
			"name":       item.ProductName,
			"price":      item.ProductPrice,
		})
	}
	return map[string]any{
		"user_id": cart.UserID,
		"items":   items,
	}
}

func mapOrder(o *domorder.Order) map[string]any {
	items := make([]map[string]any, 0, len(o.Items))
	for _, item := range o.Items {
		items = append(items, map[string]any{
			"product_id": item.ProductID,
			"name":       item.Name,
			"price":      item.Price,
			"quantity":   item.Quantity,
		})
	}

	return map[string]any{
		"id":             o.ID,
		"user_id":        o.UserID,
		"status":         o.Status,
		"payment_method": o.PaymentMethod,
		"total_amount":   o.TotalAmount,
		"created_at":     o.CreatedAt,
		"items":          items,
	}
}

func handleDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domuser.ErrCannotAssignRole),
		errors.Is(err, domuser.ErrInvalidRoleCode),
		errors.Is(err, domuser.ErrInvalidCredential),
		errors.Is(err, domuser.ErrAdminCannotCreateAdmin),
		errors.Is(err, domuser.ErrAdminCannotPromoteAdmin):
		respondError(w, http.StatusUnprocessableEntity, err)
	case errors.Is(err, domcategory.ErrCategoryInvalidName),
		errors.Is(err, domcategory.ErrCategoryInvalidSlug):
		respondError(w, http.StatusUnprocessableEntity, err)
	case errors.Is(err, domcategory.ErrCategorySlugExists),
		errors.Is(err, domrole.ErrRoleCodeExisted),
		errors.Is(err, domuser.ErrEmailAlreadyUsed):
		respondError(w, http.StatusConflict, err)
	case errors.Is(err, domuser.ErrUserNotFound),
		errors.Is(err, domrole.ErrRoleNotFound),
		errors.Is(err, domcategory.ErrCategoryNotFound),
		errors.Is(err, domproduct.ErrProductNotFound),
		errors.Is(err, domorder.ErrOrderNotFound):
		respondError(w, http.StatusNotFound, err)
	case errors.Is(err, domuser.ErrUnauthorized):
		respondError(w, http.StatusUnauthorized, err)
	case errors.Is(err, domrole.ErrRoleImmutable),
		errors.Is(err, domrole.ErrRoleInUse),
		errors.Is(err, domorder.ErrEmptyOrderItems),
		errors.Is(err, domorder.ErrInvalidPayment),
		errors.Is(err, domorder.ErrCheckoutValidation),
		errors.Is(err, domorder.ErrInvalidStatus),
		errors.Is(err, domproduct.ErrOutOfStock):
		// Lỗi nghiệp vụ khi checkout/cart → 422
		respondError(w, http.StatusUnprocessableEntity, err)
	default:
		respondError(w, http.StatusInternalServerError, err)
	}
}
