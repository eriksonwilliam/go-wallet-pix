package application

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/eriksonwilliam/go-wallet-pix/internal/domain"
)

// IDGenerator produz identificadores unicos para novas carteiras.
type IDGenerator func() string

// Clock fornece o instante atual (injetavel para testes deterministicos).
type Clock func() time.Time

// WalletService orquestra os casos de uso. O mutex garante atomicidade das
// operacoes de escrita sobre o repositorio (relevante na transferencia, que
// toca duas carteiras).
type WalletService struct {
	repo  WalletRepository
	newID IDGenerator
	now   Clock
	mu    sync.Mutex
}

func NewWalletService(repo WalletRepository, idGen IDGenerator, clock Clock) *WalletService {
	return &WalletService{repo: repo, newID: idGen, now: clock}
}

func validAmount(amount int64) error {
	if amount <= 0 {
		return domain.ErrInvalidAmount
	}
	return nil
}

// CreateWallet cria uma carteira, garantindo unicidade da chave pix.
func (s *WalletService) CreateWallet(ctx context.Context, owner, pixKey string) (*domain.Wallet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.repo.FindByPixKey(ctx, strings.TrimSpace(pixKey))
	if err != nil && !errors.Is(err, domain.ErrWalletNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrPixKeyAlreadyInUse
	}

	wallet, err := domain.NewWallet(s.newID(), owner, pixKey)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, wallet); err != nil {
		return nil, err
	}
	return wallet, nil
}

// GetWallet recupera uma carteira por id.
func (s *WalletService) GetWallet(ctx context.Context, id string) (*domain.Wallet, error) {
	return s.repo.FindByID(ctx, id)
}

// Deposit credita um valor na carteira.
func (s *WalletService) Deposit(ctx context.Context, id string, amount int64) (*domain.Wallet, error) {
	if err := validAmount(amount); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	wallet.Deposit(amount, s.now())
	if err := s.repo.Save(ctx, wallet); err != nil {
		return nil, err
	}
	return wallet, nil
}

// Withdraw debita um valor da carteira.
func (s *WalletService) Withdraw(ctx context.Context, id string, amount int64) (*domain.Wallet, error) {
	if err := validAmount(amount); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := wallet.Withdraw(amount, s.now()); err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, wallet); err != nil {
		return nil, err
	}
	return wallet, nil
}

// Transfer move um valor da carteira de origem para a carteira destino,
// identificada pela chave pix. Retorna a carteira de origem atualizada.
func (s *WalletService) Transfer(ctx context.Context, fromID, toPixKey string, amount int64) (*domain.Wallet, error) {
	if err := validAmount(amount); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	from, err := s.repo.FindByID(ctx, fromID)
	if err != nil {
		return nil, err
	}
	to, err := s.repo.FindByPixKey(ctx, strings.TrimSpace(toPixKey))
	if err != nil {
		return nil, err
	}
	if from.ID() == to.ID() {
		return nil, domain.ErrSameWallet
	}

	at := s.now()
	if err := from.TransferOut(amount, at); err != nil {
		return nil, err
	}
	to.TransferIn(amount, at)

	if err := s.repo.Save(ctx, from); err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, to); err != nil {
		return nil, err
	}
	return from, nil
}
