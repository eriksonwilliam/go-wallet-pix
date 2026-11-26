package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/eriksonwilliam/go-wallet-pix/internal/application"
	"github.com/eriksonwilliam/go-wallet-pix/internal/domain"
)

// Handler expoe os casos de uso da carteira via HTTP.
type Handler struct {
	service *application.WalletService
}

func NewHandler(service *application.WalletService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) createWallet(w http.ResponseWriter, r *http.Request) {
	var req createWalletRequest
	if !decode(w, r, &req) {
		return
	}
	wallet, err := h.service.CreateWallet(r.Context(), req.Owner, req.PixKey)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toWalletResponse(wallet))
}

func (h *Handler) getWallet(w http.ResponseWriter, r *http.Request) {
	wallet, err := h.service.GetWallet(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toWalletResponse(wallet))
}

func (h *Handler) deposit(w http.ResponseWriter, r *http.Request) {
	var req amountRequest
	if !decode(w, r, &req) {
		return
	}
	wallet, err := h.service.Deposit(r.Context(), idempotencyKey(r), r.PathValue("id"), req.AmountCents)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toWalletResponse(wallet))
}

func (h *Handler) withdraw(w http.ResponseWriter, r *http.Request) {
	var req amountRequest
	if !decode(w, r, &req) {
		return
	}
	wallet, err := h.service.Withdraw(r.Context(), idempotencyKey(r), r.PathValue("id"), req.AmountCents)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toWalletResponse(wallet))
}

func (h *Handler) transfer(w http.ResponseWriter, r *http.Request) {
	var req transferRequest
	if !decode(w, r, &req) {
		return
	}
	wallet, err := h.service.Transfer(r.Context(), idempotencyKey(r), r.PathValue("id"), req.ToPixKey, req.AmountCents)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toWalletResponse(wallet))
}

// idempotencyKey le o header opcional. Sem ele a operacao nao e deduplicada.
func idempotencyKey(r *http.Request) string {
	return r.Header.Get("Idempotency-Key")
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// decode le o corpo JSON; em caso de erro responde 400 e retorna false.
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "corpo da requisicao invalido"})
		return false
	}
	return true
}

// writeError traduz erros de dominio em status HTTP.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrWalletNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrInsufficientFunds),
		errors.Is(err, domain.ErrPixKeyAlreadyInUse),
		errors.Is(err, domain.ErrBalanceOverflow),
		errors.Is(err, application.ErrIdempotencyConflict):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrInvalidAmount),
		errors.Is(err, domain.ErrInvalidOwner),
		errors.Is(err, domain.ErrInvalidPixKey),
		errors.Is(err, domain.ErrSameWallet):
		status = http.StatusBadRequest
	}
	writeJSON(w, status, errorResponse{Error: err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
