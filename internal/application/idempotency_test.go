package application_test

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"

	"github.com/eriksonwilliam/go-wallet-pix/internal/application"
	"github.com/eriksonwilliam/go-wallet-pix/internal/domain"
	"github.com/eriksonwilliam/go-wallet-pix/internal/infrastructure/memory"
)

// fakeIdempotency guarda em mapa e permite injetar falha em Find/Remember.
type fakeIdempotency struct {
	results     map[string]application.IdempotentResult
	findErr     error
	rememberErr error
}

func newIdempotency() *fakeIdempotency {
	return &fakeIdempotency{results: map[string]application.IdempotentResult{}}
}

func (f *fakeIdempotency) Find(_ context.Context, key string) (application.IdempotentResult, bool, error) {
	if f.findErr != nil {
		return application.IdempotentResult{}, false, f.findErr
	}
	result, ok := f.results[key]
	return result, ok, nil
}

func (f *fakeIdempotency) Remember(_ context.Context, key string, result application.IdempotentResult) error {
	if f.rememberErr != nil {
		return f.rememberErr
	}
	f.results[key] = result
	return nil
}

func realService(t *testing.T) (*application.WalletService, *domain.Wallet, *domain.Wallet) {
	t.Helper()
	ids := []string{"id-alice", "id-bob"}
	next := 0
	svc := application.NewWalletService(memory.New(), memory.NewIdempotencyStore(), func() string {
		id := ids[next]
		next++
		return id
	}, fixedClock)
	ctx := context.Background()
	alice, err := svc.CreateWallet(ctx, "Alice", "alice@pix")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := svc.CreateWallet(ctx, "Bob", "bob@pix")
	if err != nil {
		t.Fatal(err)
	}
	return svc, alice, bob
}

func TestIdempotentRetryAppliesOnce(t *testing.T) {
	ctx := context.Background()
	svc, alice, _ := realService(t)

	for attempt := 0; attempt < 3; attempt++ {
		w, err := svc.Deposit(ctx, "dep-1", alice.ID(), 1000)
		if err != nil || w.Balance() != 1000 {
			t.Fatalf("tentativa %d: %v / %v", attempt, w, err)
		}
	}
	for attempt := 0; attempt < 3; attempt++ {
		if _, err := svc.Transfer(ctx, "tx-1", alice.ID(), "bob@pix", 300); err != nil {
			t.Fatalf("tentativa %d: %v", attempt, err)
		}
	}
	for attempt := 0; attempt < 3; attempt++ {
		if _, err := svc.Withdraw(ctx, "wd-1", alice.ID(), 100); err != nil {
			t.Fatalf("tentativa %d: %v", attempt, err)
		}
	}

	got, _ := svc.GetWallet(ctx, alice.ID())
	bob, _ := svc.GetWallet(ctx, "id-bob")
	if got.Balance() != 600 || bob.Balance() != 300 {
		t.Fatalf("retries aplicaram mais de uma vez: alice %d, bob %d", got.Balance(), bob.Balance())
	}
	if len(got.Transactions()) != 3 {
		t.Fatalf("extrato com lancamentos duplicados: %+v", got.Transactions())
	}
}

func TestIdempotentReplayReturnsOriginalResult(t *testing.T) {
	ctx := context.Background()
	svc, alice, _ := realService(t)

	first, _ := svc.Deposit(ctx, "dep-1", alice.ID(), 1000)
	if _, err := svc.Deposit(ctx, "dep-2", alice.ID(), 500); err != nil {
		t.Fatal(err)
	}
	replay, err := svc.Deposit(ctx, "dep-1", alice.ID(), 1000)

	if err != nil || replay.Balance() != first.Balance() {
		t.Fatalf("replay deveria devolver o resultado original (%d), obtive %v / %v",
			first.Balance(), replay, err)
	}
}

func TestIdempotencyKeyReusedWithOtherRequestConflicts(t *testing.T) {
	ctx := context.Background()
	svc, alice, _ := realService(t)
	if _, err := svc.Deposit(ctx, "k", alice.ID(), 1000); err != nil {
		t.Fatal(err)
	}

	cases := map[string]func() error{
		"outro valor":    func() error { _, err := svc.Deposit(ctx, "k", alice.ID(), 999); return err },
		"outra operacao": func() error { _, err := svc.Withdraw(ctx, "k", alice.ID(), 1000); return err },
		"outro destino": func() error {
			_, err := svc.Transfer(ctx, "k", alice.ID(), "bob@pix", 1000)
			return err
		},
	}
	for name, call := range cases {
		if err := call(); !errors.Is(err, application.ErrIdempotencyConflict) {
			t.Fatalf("%s: esperava ErrIdempotencyConflict, obtive %v", name, err)
		}
	}
	if application.ErrIdempotencyConflict.Error() == "" {
		t.Fatal("erro sem mensagem")
	}
}

func TestFailedOperationIsNotRemembered(t *testing.T) {
	ctx := context.Background()
	svc, alice, _ := realService(t)

	if _, err := svc.Withdraw(ctx, "wd", alice.ID(), 500); !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("esperava ErrInsufficientFunds, obtive %v", err)
	}
	if _, err := svc.Deposit(ctx, "", alice.ID(), 500); err != nil {
		t.Fatal(err)
	}
	w, err := svc.Withdraw(ctx, "wd", alice.ID(), 500)
	if err != nil || w.Balance() != 0 {
		t.Fatalf("a mesma chave deveria funcionar depois de uma falha: %v / %v", w, err)
	}
}

func TestIdempotencyStoreFailures(t *testing.T) {
	ctx := context.Background()
	wallet := func(context.Context, string) (*domain.Wallet, error) {
		return newWallet(t, "id-1", "Alice", "alice@pix"), nil
	}

	findFails := newIdempotency()
	findFails.findErr = boom
	svc := application.NewWalletService(&fakeRepo{findByIDFn: wallet, saveFn: okSave}, findFails, seqID(), fixedClock)
	if _, err := svc.Deposit(ctx, "k", "id-1", 100); !errors.Is(err, boom) {
		t.Fatalf("esperava boom no Find, obtive %v", err)
	}

	rememberFails := newIdempotency()
	rememberFails.rememberErr = boom
	svc = application.NewWalletService(&fakeRepo{findByIDFn: wallet, saveFn: okSave}, rememberFails, seqID(), fixedClock)
	if _, err := svc.Deposit(ctx, "k", "id-1", 100); !errors.Is(err, boom) {
		t.Fatalf("esperava boom no Remember, obtive %v", err)
	}
}

func TestDepositOverflowIsRejected(t *testing.T) {
	ctx := context.Background()
	svc, alice, _ := realService(t)
	if _, err := svc.Deposit(ctx, "", alice.ID(), math.MaxInt64); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Deposit(ctx, "", alice.ID(), 1); !errors.Is(err, domain.ErrBalanceOverflow) {
		t.Fatalf("esperava ErrBalanceOverflow, obtive %v", err)
	}
	got, _ := svc.GetWallet(ctx, alice.ID())
	if got.Balance() != math.MaxInt64 {
		t.Fatalf("saldo alterado por deposito rejeitado: %d", got.Balance())
	}
}

func TestTransferSavesBothWalletsInOneCall(t *testing.T) {
	ctx := context.Background()
	var saved [][]*domain.Wallet
	repo := &fakeRepo{
		findByIDFn: func(context.Context, string) (*domain.Wallet, error) {
			w := newWallet(t, "id-from", "Alice", "alice@pix")
			_ = w.Deposit(1000, fixedClock())
			return w, nil
		},
		findByPixFn: func(context.Context, string) (*domain.Wallet, error) {
			return newWallet(t, "id-to", "Bob", "bob@pix"), nil
		},
		saveFn: func(_ context.Context, wallets ...*domain.Wallet) error {
			saved = append(saved, wallets)
			return nil
		},
	}
	svc := application.NewWalletService(repo, newIdempotency(), seqID(), fixedClock)

	if _, err := svc.Transfer(ctx, "", "id-from", " bob@pix ", 300); err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 || len(saved[0]) != 2 {
		t.Fatalf("esperava um unico Save com as duas carteiras, obtive %d chamadas", len(saved))
	}
	if saved[0][0].Balance() != 700 || saved[0][1].Balance() != 300 {
		t.Fatalf("saldos gravados: %d / %d", saved[0][0].Balance(), saved[0][1].Balance())
	}
}

// Propriedade: transferencias concorrentes em sentidos opostos conservam o
// total e nunca deixam saldo negativo. Rode com -race.
func TestConcurrentTransfersConserveMoney(t *testing.T) {
	ctx := context.Background()
	svc, alice, bob := realService(t)
	_, _ = svc.Deposit(ctx, "", alice.ID(), 10_000)
	_, _ = svc.Deposit(ctx, "", bob.ID(), 10_000)

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(3)
		go func() { defer wg.Done(); _, _ = svc.Transfer(ctx, "", alice.ID(), "bob@pix", 70) }()
		go func() { defer wg.Done(); _, _ = svc.Transfer(ctx, "", bob.ID(), "alice@pix", 90) }()
		go func() {
			defer wg.Done()
			w, err := svc.GetWallet(ctx, alice.ID())
			if err == nil {
				_ = w.Transactions()
			}
		}()
	}
	wg.Wait()

	a, _ := svc.GetWallet(ctx, alice.ID())
	b, _ := svc.GetWallet(ctx, bob.ID())
	if a.Balance() < 0 || b.Balance() < 0 {
		t.Fatalf("saldo negativo: %d / %d", a.Balance(), b.Balance())
	}
	if a.Balance()+b.Balance() != 20_000 {
		t.Fatalf("dinheiro criado ou destruido: total %d", a.Balance()+b.Balance())
	}
}
