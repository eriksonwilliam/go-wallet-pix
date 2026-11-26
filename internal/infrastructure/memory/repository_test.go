package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eriksonwilliam/go-wallet-pix/internal/application"
	"github.com/eriksonwilliam/go-wallet-pix/internal/domain"
	"github.com/eriksonwilliam/go-wallet-pix/internal/infrastructure/memory"
)

func TestRepositorySaveAndFind(t *testing.T) {
	ctx := context.Background()
	repo := memory.New()
	wallet, _ := domain.NewWallet("id-1", "Alice", "alice@pix")

	if err := repo.Save(ctx, wallet); err != nil {
		t.Fatalf("nao esperava erro no save: %v", err)
	}

	byID, err := repo.FindByID(ctx, "id-1")
	if err != nil || byID.ID() != "id-1" {
		t.Fatalf("FindByID falhou: %v / %v", byID, err)
	}

	byPix, err := repo.FindByPixKey(ctx, "alice@pix")
	if err != nil || byPix.ID() != "id-1" {
		t.Fatalf("FindByPixKey falhou: %v / %v", byPix, err)
	}
}

func TestRepositoryNotFound(t *testing.T) {
	ctx := context.Background()
	repo := memory.New()

	if _, err := repo.FindByID(ctx, "nao-existe"); !errors.Is(err, domain.ErrWalletNotFound) {
		t.Fatalf("esperava ErrWalletNotFound, obtive %v", err)
	}
	if _, err := repo.FindByPixKey(ctx, "nao-existe"); !errors.Is(err, domain.ErrWalletNotFound) {
		t.Fatalf("esperava ErrWalletNotFound, obtive %v", err)
	}
}

func TestRepositoryReturnsIndependentCopies(t *testing.T) {
	ctx := context.Background()
	repo := memory.New()
	wallet, _ := domain.NewWallet("id-1", "Alice", "alice@pix")
	_ = repo.Save(ctx, wallet)

	_ = wallet.Deposit(100, time.Now())
	loaded, _ := repo.FindByID(ctx, "id-1")
	if loaded.Balance() != 0 {
		t.Fatalf("alterar a carteira depois do Save mudou o estado guardado: %d", loaded.Balance())
	}

	_ = loaded.Deposit(50, time.Now())
	again, _ := repo.FindByPixKey(ctx, "alice@pix")
	if again.Balance() != 0 {
		t.Fatalf("alterar a carteira lida mudou o estado guardado: %d", again.Balance())
	}
}

func TestRepositorySavesSeveralWallets(t *testing.T) {
	ctx := context.Background()
	repo := memory.New()
	alice, _ := domain.NewWallet("id-1", "Alice", "alice@pix")
	bob, _ := domain.NewWallet("id-2", "Bob", "bob@pix")

	if err := repo.Save(ctx, alice, bob); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindByPixKey(ctx, "bob@pix"); err != nil {
		t.Fatalf("segunda carteira nao gravada: %v", err)
	}
}

func TestIdempotencyStore(t *testing.T) {
	ctx := context.Background()
	store := memory.NewIdempotencyStore()

	if _, found, err := store.Find(ctx, "k"); found || err != nil {
		t.Fatalf("chave inexistente: found=%v err=%v", found, err)
	}
	wallet, _ := domain.NewWallet("id-1", "Alice", "alice@pix")
	if err := store.Remember(ctx, "k", application.IdempotentResult{Fingerprint: "f", Wallet: wallet}); err != nil {
		t.Fatal(err)
	}
	result, found, err := store.Find(ctx, "k")
	if !found || err != nil || result.Fingerprint != "f" || result.Wallet.ID() != "id-1" {
		t.Fatalf("resultado inesperado: %+v found=%v err=%v", result, found, err)
	}
}
