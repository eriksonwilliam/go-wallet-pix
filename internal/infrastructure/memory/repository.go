package memory

import (
	"context"
	"sync"

	"github.com/eriksonwilliam/go-wallet-pix/internal/application"
	"github.com/eriksonwilliam/go-wallet-pix/internal/domain"
)

// WalletRepository e um adapter de persistencia em memoria, seguro para uso
// concorrente. Substituivel por Postgres/Redis sem tocar na aplicacao.
//
// Guarda e devolve copias. Devolver o ponteiro guardado fazia o handler HTTP
// serializar a carteira enquanto outra requisicao a alterava (corrida
// detectada por go test -race), e uma alteracao feita antes de um Save que
// falhasse ja aparecia no estado "persistido".
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

func (r *WalletRepository) Save(_ context.Context, wallets ...*domain.Wallet) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, wallet := range wallets {
		r.byID[wallet.ID()] = wallet.Clone()
		r.byPix[wallet.PixKey()] = wallet.ID()
	}
	return nil
}

func (r *WalletRepository) FindByID(_ context.Context, id string) (*domain.Wallet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	wallet, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrWalletNotFound
	}
	return wallet.Clone(), nil
}

func (r *WalletRepository) FindByPixKey(_ context.Context, pixKey string) (*domain.Wallet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byPix[pixKey]
	if !ok {
		return nil, domain.ErrWalletNotFound
	}
	return r.byID[id].Clone(), nil
}

// IdempotencyStore guarda o resultado de cada Idempotency-Key em memoria.
// Nao expira chaves: num processo de longa duracao, um adapter real precisa
// de TTL.
type IdempotencyStore struct {
	mu      sync.Mutex
	results map[string]application.IdempotentResult
}

func NewIdempotencyStore() *IdempotencyStore {
	return &IdempotencyStore{results: make(map[string]application.IdempotentResult)}
}

func (s *IdempotencyStore) Find(_ context.Context, key string) (application.IdempotentResult, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.results[key]
	return result, ok, nil
}

func (s *IdempotencyStore) Remember(_ context.Context, key string, result application.IdempotentResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[key] = result
	return nil
}
