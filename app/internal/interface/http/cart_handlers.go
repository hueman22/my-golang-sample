package http

import (
	"net/http"

	domorder "example.com/my-golang-sample/app/internal/domain/order"
)

type addCartItemRequest struct {
	ProductID int64 `json:"product_id" validate:"required,gt=0"`
	Quantity  int64 `json:"quantity" validate:"required,gt=0"`
}

type checkoutRequest struct {
	PaymentMethod string `json:"payment_method" validate:"required"`
}

func (a *API) handleAddCartItem(w http.ResponseWriter, r *http.Request) {
	user := getAuthUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, errUnauthenticated)
		return
	}

	var req addCartItemRequest
	if err := a.decodeAndValidate(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	if err := a.cartSvc.AddToCart(r.Context(), user.UserID, req.ProductID, req.Quantity); err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "added"})
}

func (a *API) handleGetCart(w http.ResponseWriter, r *http.Request) {
	user := getAuthUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, errUnauthenticated)
		return
	}

	cart, err := a.cartSvc.GetCart(r.Context(), user.UserID)
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapCart(cart))
}

func (a *API) handleCheckout(w http.ResponseWriter, r *http.Request) {
	user := getAuthUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, errUnauthenticated)
		return
	}

	var req checkoutRequest
	if err := a.decodeAndValidate(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	method := domorder.PaymentMethod(req.PaymentMethod)
	order, err := a.cartSvc.Checkout(r.Context(), user.UserID, method)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, mapOrder(order))
}

