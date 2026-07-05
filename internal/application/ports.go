package application

import (
	"context"

	"github.com/eriksonwilliam/go-wallet-pix/internal/domain"
)

// WalletRepository e o port de saida para persistencia de carteiras.
// FindByID e FindByPixKey devem retornar domain.ErrWalletNotFound quando
// nao encontram a carteira.
type WalletRepository interface {
	Save(ctx context.Context, wallet *domain.Wallet) error
	FindByID(ctx context.Context, id string) (*domain.Wallet, error)
	FindByPixKey(ctx context.Context, pixKey string) (*domain.Wallet, error)
}
