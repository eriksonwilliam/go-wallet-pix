package domain

import (
	"math"
	"strings"
	"time"
)

// TransactionType identifica a natureza de um lancamento no extrato.
type TransactionType string

const (
	Deposit     TransactionType = "deposit"
	Withdraw    TransactionType = "withdraw"
	TransferIn  TransactionType = "transfer_in"
	TransferOut TransactionType = "transfer_out"
)

// Transaction e um lancamento imutavel no extrato da carteira.
// Os valores sao sempre em centavos, evitando imprecisao de ponto flutuante.
type Transaction struct {
	Type      TransactionType
	Amount    int64
	CreatedAt time.Time
}

// Wallet e o aggregate root. Invariante central: o saldo nunca fica negativo.
// A validacao de "valor positivo" pertence a borda (aplicacao/HTTP), enquanto
// a garantia de saldo suficiente e uma regra do proprio dominio.
type Wallet struct {
	id           string
	owner        string
	pixKey       string
	balance      int64
	transactions []Transaction
}

// NewWallet cria uma carteira validando titular e chave pix.
func NewWallet(id, owner, pixKey string) (*Wallet, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return nil, ErrInvalidOwner
	}
	pixKey = strings.TrimSpace(pixKey)
	if pixKey == "" {
		return nil, ErrInvalidPixKey
	}
	return &Wallet{id: id, owner: owner, pixKey: pixKey}, nil
}

func (w *Wallet) ID() string     { return w.id }
func (w *Wallet) Owner() string  { return w.owner }
func (w *Wallet) PixKey() string { return w.pixKey }
func (w *Wallet) Balance() int64 { return w.balance }

// Transactions retorna uma copia defensiva do extrato.
func (w *Wallet) Transactions() []Transaction {
	return append([]Transaction(nil), w.transactions...)
}

// Clone devolve uma copia independente, inclusive do extrato. Quem guarda
// carteiras (repositorio, registro de idempotencia) guarda copias: alterar a
// carteira recebida nao pode mudar o estado guardado antes do Save.
func (w *Wallet) Clone() *Wallet {
	clone := *w
	clone.transactions = w.Transactions()
	return &clone
}

// Deposit credita um valor na carteira.
func (w *Wallet) Deposit(amount int64, at time.Time) error {
	return w.credit(Deposit, amount, at)
}

// Withdraw debita um valor, respeitando o saldo disponivel.
func (w *Wallet) Withdraw(amount int64, at time.Time) error {
	return w.debit(Withdraw, amount, at)
}

// Transfer move um valor entre duas carteiras. Valida as duas pontas antes de
// alterar qualquer uma: ou as duas mudam, ou nenhuma.
func Transfer(from, to *Wallet, amount int64, at time.Time) error {
	if amount > from.balance {
		return ErrInsufficientFunds
	}
	if !to.canCredit(amount) {
		return ErrBalanceOverflow
	}
	from.balance -= amount
	from.record(TransferOut, amount, at)
	to.balance += amount
	to.record(TransferIn, amount, at)
	return nil
}

// canCredit evita o overflow de int64: sem a checagem, MaxInt64 + 1 vira um
// saldo negativo e quebra o invariante central da carteira.
func (w *Wallet) canCredit(amount int64) bool {
	return amount <= math.MaxInt64-w.balance
}

func (w *Wallet) credit(t TransactionType, amount int64, at time.Time) error {
	if !w.canCredit(amount) {
		return ErrBalanceOverflow
	}
	w.balance += amount
	w.record(t, amount, at)
	return nil
}

func (w *Wallet) debit(t TransactionType, amount int64, at time.Time) error {
	if amount > w.balance {
		return ErrInsufficientFunds
	}
	w.balance -= amount
	w.record(t, amount, at)
	return nil
}

func (w *Wallet) record(t TransactionType, amount int64, at time.Time) {
	w.transactions = append(w.transactions, Transaction{Type: t, Amount: amount, CreatedAt: at})
}
