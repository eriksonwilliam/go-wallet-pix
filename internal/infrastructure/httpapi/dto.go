package httpapi

import (
	"fmt"
	"time"

	"github.com/eriksonwilliam/go-wallet-pix/internal/domain"
)

type createWalletRequest struct {
	Owner  string `json:"owner"`
	PixKey string `json:"pix_key"`
}

type amountRequest struct {
	AmountCents int64 `json:"amount_cents"`
}

type transferRequest struct {
	ToPixKey    string `json:"to_pix_key"`
	AmountCents int64  `json:"amount_cents"`
}

type transactionResponse struct {
	Type        string    `json:"type"`
	AmountCents int64     `json:"amount_cents"`
	CreatedAt   time.Time `json:"created_at"`
}

type walletResponse struct {
	ID           string                `json:"id"`
	Owner        string                `json:"owner"`
	PixKey       string                `json:"pix_key"`
	BalanceCents int64                 `json:"balance_cents"`
	Balance      string                `json:"balance"`
	Transactions []transactionResponse `json:"transactions"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func toWalletResponse(wallet *domain.Wallet) walletResponse {
	txs := wallet.Transactions()
	items := make([]transactionResponse, 0, len(txs))
	for _, t := range txs {
		items = append(items, transactionResponse{
			Type:        string(t.Type),
			AmountCents: t.Amount,
			CreatedAt:   t.CreatedAt,
		})
	}
	return walletResponse{
		ID:           wallet.ID(),
		Owner:        wallet.Owner(),
		PixKey:       wallet.PixKey(),
		BalanceCents: wallet.Balance(),
		Balance:      formatBRL(wallet.Balance()),
		Transactions: items,
	}
}

// formatBRL formata centavos como moeda brasileira (ex.: 12345 -> "R$ 123,45").
func formatBRL(cents int64) string {
	return fmt.Sprintf("R$ %d,%02d", cents/100, cents%100)
}
