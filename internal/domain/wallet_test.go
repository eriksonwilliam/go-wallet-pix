package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/eriksonwilliam/go-wallet-pix/internal/domain"
)

var at = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func TestNewWalletValidations(t *testing.T) {
	if _, err := domain.NewWallet("id", "  ", "chave"); !errors.Is(err, domain.ErrInvalidOwner) {
		t.Fatalf("esperava ErrInvalidOwner, obtive %v", err)
	}
	if _, err := domain.NewWallet("id", "Alice", "   "); !errors.Is(err, domain.ErrInvalidPixKey) {
		t.Fatalf("esperava ErrInvalidPixKey, obtive %v", err)
	}

	wallet, err := domain.NewWallet("id-1", "  Alice  ", "  alice@pix  ")
	if err != nil {
		t.Fatalf("nao esperava erro: %v", err)
	}
	if wallet.Owner() != "Alice" || wallet.PixKey() != "alice@pix" {
		t.Fatalf("titular/chave nao normalizados: %q / %q", wallet.Owner(), wallet.PixKey())
	}
	if wallet.ID() != "id-1" || wallet.Balance() != 0 {
		t.Fatalf("estado inicial invalido")
	}
}

func TestDepositAndWithdraw(t *testing.T) {
	wallet, _ := domain.NewWallet("id-1", "Alice", "alice@pix")

	wallet.Deposit(10000, at)
	if wallet.Balance() != 10000 {
		t.Fatalf("saldo apos deposito = %d", wallet.Balance())
	}

	if err := wallet.Withdraw(3000, at); err != nil {
		t.Fatalf("nao esperava erro no saque: %v", err)
	}
	if wallet.Balance() != 7000 {
		t.Fatalf("saldo apos saque = %d", wallet.Balance())
	}

	if err := wallet.Withdraw(999999, at); !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("esperava ErrInsufficientFunds, obtive %v", err)
	}
}

func TestTransferHelpers(t *testing.T) {
	from, _ := domain.NewWallet("id-1", "Alice", "alice@pix")
	to, _ := domain.NewWallet("id-2", "Bob", "bob@pix")

	from.Deposit(5000, at)
	if err := from.TransferOut(2000, at); err != nil {
		t.Fatalf("nao esperava erro: %v", err)
	}
	to.TransferIn(2000, at)

	if from.Balance() != 3000 || to.Balance() != 2000 {
		t.Fatalf("saldos incorretos: %d / %d", from.Balance(), to.Balance())
	}
	if err := from.TransferOut(999999, at); !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("esperava ErrInsufficientFunds, obtive %v", err)
	}
}

func TestTransactionsAreCopied(t *testing.T) {
	wallet, _ := domain.NewWallet("id-1", "Alice", "alice@pix")
	wallet.Deposit(100, at)

	txs := wallet.Transactions()
	if len(txs) != 1 || txs[0].Type != domain.Deposit || txs[0].Amount != 100 {
		t.Fatalf("extrato inesperado: %+v", txs)
	}

	txs[0].Amount = 999
	if wallet.Transactions()[0].Amount != 100 {
		t.Fatalf("mutacao externa vazou para o dominio")
	}
}
