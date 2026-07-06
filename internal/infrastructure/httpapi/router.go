package httpapi

import "net/http"

// NewRouter registra as rotas usando o roteamento por metodo do net/http
// (Go 1.22+), sem dependencias externas.
func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("GET /openapi.json", openAPIHandler)
	mux.HandleFunc("GET /docs", docsHandler)
	mux.HandleFunc("POST /wallets", h.createWallet)
	mux.HandleFunc("GET /wallets/{id}", h.getWallet)
	mux.HandleFunc("POST /wallets/{id}/deposit", h.deposit)
	mux.HandleFunc("POST /wallets/{id}/withdraw", h.withdraw)
	mux.HandleFunc("POST /wallets/{id}/transfer", h.transfer)
	return mux
}
