package domain_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/eriksonwilliam/go-wallet-pix/internal/domain"
)

var at = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func mustWallet(t *testing.T, id, owner, pix string) *domain.Wallet {
	t.Helper()
	wallet, err := domain.NewWallet(id, owner, pix)
	if err != nil {
		t.Fatalf("nao esperava erro: %v", err)
	}
	return wallet
}

func mustDeposit(t *testing.T, wallet *domain.Wallet, amount int64) {
	t.Helper()
	if err := wallet.Deposit(amount, at); err != nil {
		t.Fatalf("nao esperava erro no deposito: %v", err)
	}
}

func TestNewWalletValidations(t *testing.T) {
	if _, err := domain.NewWallet("id", "  ", "chave"); !errors.Is(err, domain.ErrInvalidOwner) {
		t.Fatalf("esperava ErrInvalidOwner, obtive %v", err)
	}
	if _, err := domain.NewWallet("id", "Alice", "   "); !errors.Is(err, domain.ErrInvalidPixKey) {
		t.Fatalf("esperava ErrInvalidPixKey, obtive %v", err)
	}

	wallet := mustWallet(t, "id-1", "  Alice  ", "  alice@pix  ")
	if wallet.Owner() != "Alice" || wallet.PixKey() != "alice@pix" {
		t.Fatalf("titular/chave nao normalizados: %q / %q", wallet.Owner(), wallet.PixKey())
	}
	if wallet.ID() != "id-1" || wallet.Balance() != 0 {
		t.Fatalf("estado inicial invalido")
	}
}

func TestDepositAndWithdraw(t *testing.T) {
	wallet := mustWallet(t, "id-1", "Alice", "alice@pix")

	mustDeposit(t, wallet, 10000)
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

func TestDepositRejectsOverflow(t *testing.T) {
	wallet := mustWallet(t, "id-1", "Alice", "alice@pix")
	mustDeposit(t, wallet, math.MaxInt64)

	if err := wallet.Deposit(1, at); !errors.Is(err, domain.ErrBalanceOverflow) {
		t.Fatalf("esperava ErrBalanceOverflow, obtive %v", err)
	}
	if wallet.Balance() != math.MaxInt64 || len(wallet.Transactions()) != 1 {
		t.Fatalf("deposito rejeitado alterou a carteira: saldo %d", wallet.Balance())
	}
}

func TestTransferMovesBetweenWallets(t *testing.T) {
	from := mustWallet(t, "id-1", "Alice", "alice@pix")
	to := mustWallet(t, "id-2", "Bob", "bob@pix")
	mustDeposit(t, from, 5000)

	if err := domain.Transfer(from, to, 2000, at); err != nil {
		t.Fatalf("nao esperava erro: %v", err)
	}
	if from.Balance() != 3000 || to.Balance() != 2000 {
		t.Fatalf("saldos incorretos: %d / %d", from.Balance(), to.Balance())
	}
	if from.Transactions()[1].Type != domain.TransferOut || to.Transactions()[0].Type != domain.TransferIn {
		t.Fatalf("extrato sem os lancamentos de transferencia")
	}
}

func TestTransferIsAllOrNothing(t *testing.T) {
	t.Run("saldo insuficiente nao toca o destino", func(t *testing.T) {
		from := mustWallet(t, "id-1", "Alice", "alice@pix")
		to := mustWallet(t, "id-2", "Bob", "bob@pix")

		if err := domain.Transfer(from, to, 1, at); !errors.Is(err, domain.ErrInsufficientFunds) {
			t.Fatalf("esperava ErrInsufficientFunds, obtive %v", err)
		}
		if to.Balance() != 0 || len(to.Transactions()) != 0 {
			t.Fatalf("destino alterado por transferencia rejeitada")
		}
	})

	t.Run("overflow no destino nao debita a origem", func(t *testing.T) {
		from := mustWallet(t, "id-1", "Alice", "alice@pix")
		to := mustWallet(t, "id-2", "Bob", "bob@pix")
		mustDeposit(t, from, 10)
		mustDeposit(t, to, math.MaxInt64)

		if err := domain.Transfer(from, to, 10, at); !errors.Is(err, domain.ErrBalanceOverflow) {
			t.Fatalf("esperava ErrBalanceOverflow, obtive %v", err)
		}
		if from.Balance() != 10 || len(from.Transactions()) != 1 {
			t.Fatalf("origem debitada por transferencia rejeitada: saldo %d", from.Balance())
		}
	})
}

func TestCloneIsIndependent(t *testing.T) {
	original := mustWallet(t, "id-1", "Alice", "alice@pix")
	mustDeposit(t, original, 100)

	clone := original.Clone()
	mustDeposit(t, clone, 50)

	if original.Balance() != 100 || len(original.Transactions()) != 1 {
		t.Fatalf("alterar o clone mudou o original")
	}
	if clone.Balance() != 150 || clone.ID() != original.ID() || clone.PixKey() != original.PixKey() {
		t.Fatalf("clone inconsistente: %+v", clone)
	}
}

func TestTransactionsAreCopied(t *testing.T) {
	wallet := mustWallet(t, "id-1", "Alice", "alice@pix")
	mustDeposit(t, wallet, 100)

	txs := wallet.Transactions()
	if len(txs) != 1 || txs[0].Type != domain.Deposit || txs[0].Amount != 100 {
		t.Fatalf("extrato inesperado: %+v", txs)
	}

	txs[0].Amount = 999
	if wallet.Transactions()[0].Amount != 100 {
		t.Fatalf("mutacao externa vazou para o dominio")
	}
}

func TestDomainErrorsHaveMessages(t *testing.T) {
	for _, err := range []error{
		domain.ErrWalletNotFound, domain.ErrInsufficientFunds, domain.ErrInvalidAmount,
		domain.ErrInvalidOwner, domain.ErrInvalidPixKey, domain.ErrPixKeyAlreadyInUse,
		domain.ErrSameWallet, domain.ErrBalanceOverflow,
	} {
		if err.Error() == "" {
			t.Fatalf("erro sem mensagem: %#v", err)
		}
	}
}
