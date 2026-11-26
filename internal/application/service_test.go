package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eriksonwilliam/go-wallet-pix/internal/application"
	"github.com/eriksonwilliam/go-wallet-pix/internal/domain"
)

// fakeRepo permite injetar comportamento por chamada, cobrindo os ramos de
// erro do servico (falhas de persistencia, nao encontrado, etc.).
type fakeRepo struct {
	saveFn      func(ctx context.Context, w ...*domain.Wallet) error
	findByIDFn  func(ctx context.Context, id string) (*domain.Wallet, error)
	findByPixFn func(ctx context.Context, pixKey string) (*domain.Wallet, error)
}

func (r *fakeRepo) Save(ctx context.Context, w ...*domain.Wallet) error {
	return r.saveFn(ctx, w...)
}
func (r *fakeRepo) FindByID(ctx context.Context, id string) (*domain.Wallet, error) {
	return r.findByIDFn(ctx, id)
}
func (r *fakeRepo) FindByPixKey(ctx context.Context, pixKey string) (*domain.Wallet, error) {
	return r.findByPixFn(ctx, pixKey)
}

var (
	fixedClock = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	seqID      = func() application.IDGenerator {
		n := 0
		return func() string { n++; return "id-" + string(rune('0'+n)) }
	}
	boom        = errors.New("falha de infraestrutura")
	notFound    = func(context.Context, string) (*domain.Wallet, error) { return nil, domain.ErrWalletNotFound }
	notFoundPix = func(context.Context, string) (*domain.Wallet, error) { return nil, domain.ErrWalletNotFound }
	okSave      = func(context.Context, ...*domain.Wallet) error { return nil }
)

func newWallet(t *testing.T, id, owner, pix string) *domain.Wallet {
	t.Helper()
	w, err := domain.NewWallet(id, owner, pix)
	if err != nil {
		t.Fatalf("nao esperava erro criando wallet: %v", err)
	}
	return w
}

func TestCreateWallet(t *testing.T) {
	ctx := context.Background()

	t.Run("sucesso", func(t *testing.T) {
		repo := &fakeRepo{findByPixFn: notFoundPix, saveFn: okSave}
		svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
		w, err := svc.CreateWallet(ctx, "Alice", "alice@pix")
		if err != nil || w.Owner() != "Alice" {
			t.Fatalf("resultado inesperado: %v / %v", w, err)
		}
	})

	t.Run("chave pix em uso", func(t *testing.T) {
		repo := &fakeRepo{findByPixFn: func(context.Context, string) (*domain.Wallet, error) {
			return newWallet(t, "id-x", "Outro", "alice@pix"), nil
		}}
		svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.CreateWallet(ctx, "Alice", "alice@pix"); !errors.Is(err, domain.ErrPixKeyAlreadyInUse) {
			t.Fatalf("esperava ErrPixKeyAlreadyInUse, obtive %v", err)
		}
	})

	t.Run("erro ao consultar chave pix", func(t *testing.T) {
		repo := &fakeRepo{findByPixFn: func(context.Context, string) (*domain.Wallet, error) { return nil, boom }}
		svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.CreateWallet(ctx, "Alice", "alice@pix"); !errors.Is(err, boom) {
			t.Fatalf("esperava boom, obtive %v", err)
		}
	})

	t.Run("titular invalido", func(t *testing.T) {
		repo := &fakeRepo{findByPixFn: notFoundPix}
		svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.CreateWallet(ctx, "  ", "alice@pix"); !errors.Is(err, domain.ErrInvalidOwner) {
			t.Fatalf("esperava ErrInvalidOwner, obtive %v", err)
		}
	})

	t.Run("erro ao salvar", func(t *testing.T) {
		repo := &fakeRepo{findByPixFn: notFoundPix, saveFn: func(context.Context, ...*domain.Wallet) error { return boom }}
		svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.CreateWallet(ctx, "Alice", "alice@pix"); !errors.Is(err, boom) {
			t.Fatalf("esperava boom, obtive %v", err)
		}
	})
}

func TestGetWallet(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{findByIDFn: func(context.Context, string) (*domain.Wallet, error) {
		return newWallet(t, "id-1", "Alice", "alice@pix"), nil
	}}
	svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
	if w, err := svc.GetWallet(ctx, "id-1"); err != nil || w.ID() != "id-1" {
		t.Fatalf("resultado inesperado: %v / %v", w, err)
	}
}

func TestDeposit(t *testing.T) {
	ctx := context.Background()

	t.Run("valor invalido", func(t *testing.T) {
		svc := application.NewWalletService(&fakeRepo{}, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.Deposit(ctx, "", "id-1", 0); !errors.Is(err, domain.ErrInvalidAmount) {
			t.Fatalf("esperava ErrInvalidAmount, obtive %v", err)
		}
	})

	t.Run("carteira nao encontrada", func(t *testing.T) {
		svc := application.NewWalletService(&fakeRepo{findByIDFn: notFound}, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.Deposit(ctx, "", "id-1", 100); !errors.Is(err, domain.ErrWalletNotFound) {
			t.Fatalf("esperava ErrWalletNotFound, obtive %v", err)
		}
	})

	t.Run("erro ao salvar", func(t *testing.T) {
		repo := &fakeRepo{
			findByIDFn: func(context.Context, string) (*domain.Wallet, error) {
				return newWallet(t, "id-1", "Alice", "alice@pix"), nil
			},
			saveFn: func(context.Context, ...*domain.Wallet) error { return boom },
		}
		svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.Deposit(ctx, "", "id-1", 100); !errors.Is(err, boom) {
			t.Fatalf("esperava boom, obtive %v", err)
		}
	})

	t.Run("sucesso", func(t *testing.T) {
		repo := &fakeRepo{
			findByIDFn: func(context.Context, string) (*domain.Wallet, error) {
				return newWallet(t, "id-1", "Alice", "alice@pix"), nil
			},
			saveFn: okSave,
		}
		svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
		w, err := svc.Deposit(ctx, "", "id-1", 2500)
		if err != nil || w.Balance() != 2500 {
			t.Fatalf("resultado inesperado: %v / %v", w, err)
		}
	})
}

func TestWithdraw(t *testing.T) {
	ctx := context.Background()

	t.Run("valor invalido", func(t *testing.T) {
		svc := application.NewWalletService(&fakeRepo{}, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.Withdraw(ctx, "", "id-1", -1); !errors.Is(err, domain.ErrInvalidAmount) {
			t.Fatalf("esperava ErrInvalidAmount, obtive %v", err)
		}
	})

	t.Run("carteira nao encontrada", func(t *testing.T) {
		svc := application.NewWalletService(&fakeRepo{findByIDFn: notFound}, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.Withdraw(ctx, "", "id-1", 100); !errors.Is(err, domain.ErrWalletNotFound) {
			t.Fatalf("esperava ErrWalletNotFound, obtive %v", err)
		}
	})

	t.Run("saldo insuficiente", func(t *testing.T) {
		repo := &fakeRepo{findByIDFn: func(context.Context, string) (*domain.Wallet, error) {
			return newWallet(t, "id-1", "Alice", "alice@pix"), nil
		}}
		svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.Withdraw(ctx, "", "id-1", 100); !errors.Is(err, domain.ErrInsufficientFunds) {
			t.Fatalf("esperava ErrInsufficientFunds, obtive %v", err)
		}
	})

	t.Run("erro ao salvar", func(t *testing.T) {
		repo := &fakeRepo{
			findByIDFn: func(context.Context, string) (*domain.Wallet, error) {
				w := newWallet(t, "id-1", "Alice", "alice@pix")
				w.Deposit(1000, fixedClock())
				return w, nil
			},
			saveFn: func(context.Context, ...*domain.Wallet) error { return boom },
		}
		svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.Withdraw(ctx, "", "id-1", 100); !errors.Is(err, boom) {
			t.Fatalf("esperava boom, obtive %v", err)
		}
	})

	t.Run("sucesso", func(t *testing.T) {
		repo := &fakeRepo{
			findByIDFn: func(context.Context, string) (*domain.Wallet, error) {
				w := newWallet(t, "id-1", "Alice", "alice@pix")
				w.Deposit(1000, fixedClock())
				return w, nil
			},
			saveFn: okSave,
		}
		svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
		w, err := svc.Withdraw(ctx, "", "id-1", 400)
		if err != nil || w.Balance() != 600 {
			t.Fatalf("resultado inesperado: %v / %v", w, err)
		}
	})
}

func TestTransfer(t *testing.T) {
	ctx := context.Background()

	makeFrom := func(t *testing.T, balance int64) *domain.Wallet {
		w := newWallet(t, "id-from", "Alice", "alice@pix")
		if balance > 0 {
			w.Deposit(balance, fixedClock())
		}
		return w
	}
	makeTo := func(t *testing.T) *domain.Wallet { return newWallet(t, "id-to", "Bob", "bob@pix") }

	t.Run("valor invalido", func(t *testing.T) {
		svc := application.NewWalletService(&fakeRepo{}, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.Transfer(ctx, "", "id-from", "bob@pix", 0); !errors.Is(err, domain.ErrInvalidAmount) {
			t.Fatalf("esperava ErrInvalidAmount, obtive %v", err)
		}
	})

	t.Run("origem nao encontrada", func(t *testing.T) {
		svc := application.NewWalletService(&fakeRepo{findByIDFn: notFound}, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.Transfer(ctx, "", "id-from", "bob@pix", 100); !errors.Is(err, domain.ErrWalletNotFound) {
			t.Fatalf("esperava ErrWalletNotFound, obtive %v", err)
		}
	})

	t.Run("destino nao encontrado", func(t *testing.T) {
		repo := &fakeRepo{
			findByIDFn:  func(context.Context, string) (*domain.Wallet, error) { return makeFrom(t, 1000), nil },
			findByPixFn: notFoundPix,
		}
		svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.Transfer(ctx, "", "id-from", "bob@pix", 100); !errors.Is(err, domain.ErrWalletNotFound) {
			t.Fatalf("esperava ErrWalletNotFound, obtive %v", err)
		}
	})

	t.Run("mesma carteira", func(t *testing.T) {
		same := makeFrom(t, 1000)
		repo := &fakeRepo{
			findByIDFn:  func(context.Context, string) (*domain.Wallet, error) { return same, nil },
			findByPixFn: func(context.Context, string) (*domain.Wallet, error) { return same, nil },
		}
		svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.Transfer(ctx, "", "id-from", "alice@pix", 100); !errors.Is(err, domain.ErrSameWallet) {
			t.Fatalf("esperava ErrSameWallet, obtive %v", err)
		}
	})

	t.Run("saldo insuficiente", func(t *testing.T) {
		repo := &fakeRepo{
			findByIDFn:  func(context.Context, string) (*domain.Wallet, error) { return makeFrom(t, 0), nil },
			findByPixFn: func(context.Context, string) (*domain.Wallet, error) { return makeTo(t), nil },
		}
		svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.Transfer(ctx, "", "id-from", "bob@pix", 100); !errors.Is(err, domain.ErrInsufficientFunds) {
			t.Fatalf("esperava ErrInsufficientFunds, obtive %v", err)
		}
	})

	t.Run("erro ao salvar origem", func(t *testing.T) {
		repo := &fakeRepo{
			findByIDFn:  func(context.Context, string) (*domain.Wallet, error) { return makeFrom(t, 1000), nil },
			findByPixFn: func(context.Context, string) (*domain.Wallet, error) { return makeTo(t), nil },
			saveFn:      func(context.Context, ...*domain.Wallet) error { return boom },
		}
		svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
		if _, err := svc.Transfer(ctx, "", "id-from", "bob@pix", 100); !errors.Is(err, boom) {
			t.Fatalf("esperava boom, obtive %v", err)
		}
	})

	t.Run("sucesso", func(t *testing.T) {
		repo := &fakeRepo{
			findByIDFn:  func(context.Context, string) (*domain.Wallet, error) { return makeFrom(t, 1000), nil },
			findByPixFn: func(context.Context, string) (*domain.Wallet, error) { return makeTo(t), nil },
			saveFn:      okSave,
		}
		svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)
		from, err := svc.Transfer(ctx, "", "id-from", "bob@pix", 300)
		if err != nil || from.Balance() != 700 {
			t.Fatalf("resultado inesperado: %v / %v", from, err)
		}
	})
}
