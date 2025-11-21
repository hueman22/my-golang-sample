package http

import (
	"net/http"
	"strconv"

	domproduct "example.com/my-golang-sample/app/internal/domain/product"
)

func (a *API) handleListProducts(w http.ResponseWriter, r *http.Request) {
	filter := domproduct.ListFilter{
		OnlyActive: true,
		Search:     r.URL.Query().Get("q"),
	}
	if cid := r.URL.Query().Get("category_id"); cid != "" {
		if id, err := strconv.ParseInt(cid, 10, 64); err == nil {
			filter.CategoryID = &id
		}
	}

	products, err := a.productSvc.List(r.Context(), filter)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	resp := make([]map[string]any, 0, len(products))
	for _, p := range products {
		resp = append(resp, mapProduct(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": resp})
}

func (a *API) handleGetProduct(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	p, err := a.productSvc.GetByID(r.Context(), id)
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapProduct(p))
}

func (a *API) handleListProductsAdmin(w http.ResponseWriter, r *http.Request) {
	filter := domproduct.ListFilter{
		Search: r.URL.Query().Get("q"),
	}
	if cid := r.URL.Query().Get("category_id"); cid != "" {
		if id, err := strconv.ParseInt(cid, 10, 64); err == nil {
			filter.CategoryID = &id
		}
	}
	if status := r.URL.Query().Get("only_active"); status == "1" || status == "true" {
		filter.OnlyActive = true
	}

	products, err := a.productSvc.List(r.Context(), filter)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	resp := make([]map[string]any, 0, len(products))
	for _, p := range products {
		resp = append(resp, mapProduct(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": resp})
}

