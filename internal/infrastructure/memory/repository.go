package memory

import (
	"context"
	"sync"

	"github.com/eriksonwilliam/go-wallet-pix/internal/domain"
)

// WalletRepository e um adapter de persistencia em memoria, seguro para uso
// concorrente. Substituivel por Postgres/Redis sem tocar na aplicacao.
type WalletRepository struct {
	mu    sync.RWMutex
	byID  map[string]*domain.Wallet
	byPix map[string]string
}

func New() *WalletRepository {
	return &WalletRepository{
		byID:  make(map[string]*domain.Wallet),
		byPix: make(map[string]string),
	}
}

func (r *WalletRepository) Save(_ context.Context, wallet *domain.Wallet) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[wallet.ID()] = wallet
	r.byPix[wallet.PixKey()] = wallet.ID()
	return nil
}

func (r *WalletRepository) FindByID(_ context.Context, id string) (*domain.Wallet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	wallet, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrWalletNotFound
	}
	return wallet, nil
}

func (r *WalletRepository) FindByPixKey(_ context.Context, pixKey string) (*domain.Wallet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byPix[pixKey]
	if !ok {
		return nil, domain.ErrWalletNotFound
	}
	return r.byID[id], nil
}
