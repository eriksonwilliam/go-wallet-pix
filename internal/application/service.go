package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/eriksonwilliam/go-wallet-pix/internal/domain"
)

// ErrIdempotencyConflict: a mesma Idempotency-Key reaparece com outra operacao
// ou outros valores. Reexecutar seria adivinhar qual das duas o cliente quis.
var ErrIdempotencyConflict = errors.New("idempotency key reutilizada com outra requisicao")

// IDGenerator produz identificadores unicos para novas carteiras.
type IDGenerator func() string

// Clock fornece o instante atual (injetavel para testes deterministicos).
type Clock func() time.Time

// WalletService orquestra os casos de uso. O mutex serializa as escritas:
// ler, validar e gravar acontecem sem outra escrita no meio, e a checagem de
// idempotencia fica dentro da mesma secao critica.
type WalletService struct {
	repo        WalletRepository
	idempotency IdempotencyStore
	newID       IDGenerator
	now         Clock
	mu          sync.Mutex
}

func NewWalletService(repo WalletRepository, idempotency IdempotencyStore, idGen IDGenerator, clock Clock) *WalletService {
	return &WalletService{repo: repo, idempotency: idempotency, newID: idGen, now: clock}
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
func (s *WalletService) Deposit(ctx context.Context, idempotencyKey, id string, amount int64) (*domain.Wallet, error) {
	if err := validAmount(amount); err != nil {
		return nil, err
	}
	fingerprint := fmt.Sprintf("deposit|%s|%d", id, amount)
	return s.idempotent(ctx, idempotencyKey, fingerprint, func() (*domain.Wallet, error) {
		wallet, err := s.repo.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := wallet.Deposit(amount, s.now()); err != nil {
			return nil, err
		}
		return wallet, s.repo.Save(ctx, wallet)
	})
}

// Withdraw debita um valor da carteira.
func (s *WalletService) Withdraw(ctx context.Context, idempotencyKey, id string, amount int64) (*domain.Wallet, error) {
	if err := validAmount(amount); err != nil {
		return nil, err
	}
	fingerprint := fmt.Sprintf("withdraw|%s|%d", id, amount)
	return s.idempotent(ctx, idempotencyKey, fingerprint, func() (*domain.Wallet, error) {
		wallet, err := s.repo.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := wallet.Withdraw(amount, s.now()); err != nil {
			return nil, err
		}
		return wallet, s.repo.Save(ctx, wallet)
	})
}

// Transfer move um valor da carteira de origem para a carteira destino,
// identificada pela chave pix. Retorna a carteira de origem atualizada.
func (s *WalletService) Transfer(ctx context.Context, idempotencyKey, fromID, toPixKey string, amount int64) (*domain.Wallet, error) {
	if err := validAmount(amount); err != nil {
		return nil, err
	}
	toPixKey = strings.TrimSpace(toPixKey)
	fingerprint := fmt.Sprintf("transfer|%s|%s|%d", fromID, toPixKey, amount)
	return s.idempotent(ctx, idempotencyKey, fingerprint, func() (*domain.Wallet, error) {
		from, err := s.repo.FindByID(ctx, fromID)
		if err != nil {
			return nil, err
		}
		to, err := s.repo.FindByPixKey(ctx, toPixKey)
		if err != nil {
			return nil, err
		}
		if from.ID() == to.ID() {
			return nil, domain.ErrSameWallet
		}
		if err := domain.Transfer(from, to, amount, s.now()); err != nil {
			return nil, err
		}
		return from, s.repo.Save(ctx, from, to)
	})
}

// idempotent executa op uma unica vez por chave. Uma chave vazia desliga o
// mecanismo. So o sucesso fica registrado: uma falha (saldo insuficiente,
// por exemplo) pode dar certo numa nova tentativa com a mesma chave.
func (s *WalletService) idempotent(
	ctx context.Context,
	key, fingerprint string,
	op func() (*domain.Wallet, error),
) (*domain.Wallet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key != "" {
		previous, found, err := s.idempotency.Find(ctx, key)
		if err != nil {
			return nil, err
		}
		if found {
			if previous.Fingerprint != fingerprint {
				return nil, ErrIdempotencyConflict
			}
			return previous.Wallet.Clone(), nil
		}
	}

	wallet, err := op()
	if err != nil {
		return nil, err
	}
	if key != "" {
		result := IdempotentResult{Fingerprint: fingerprint, Wallet: wallet.Clone()}
		if err := s.idempotency.Remember(ctx, key, result); err != nil {
			return nil, err
		}
	}
	return wallet, nil
}
