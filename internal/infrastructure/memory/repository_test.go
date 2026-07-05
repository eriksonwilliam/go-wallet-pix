package memory_test

import (
	"context"
	"errors"
	"testing"

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
