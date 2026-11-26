package application

import (
	"context"

	"github.com/eriksonwilliam/go-wallet-pix/internal/domain"
)

// WalletRepository e o port de saida para persistencia de carteiras.
// FindByID e FindByPixKey devem retornar domain.ErrWalletNotFound quando
// nao encontram a carteira.
type WalletRepository interface {
	// Save persiste todas as carteiras ou nenhuma. A transferencia grava
	// origem e destino juntas: gravar so a origem perderia o dinheiro.
	Save(ctx context.Context, wallets ...*domain.Wallet) error
	FindByID(ctx context.Context, id string) (*domain.Wallet, error)
	FindByPixKey(ctx context.Context, pixKey string) (*domain.Wallet, error)
}

// IdempotentResult e o que fica registrado para uma Idempotency-Key: a
// identidade da requisicao original e a carteira resultante.
type IdempotentResult struct {
	Fingerprint string
	Wallet      *domain.Wallet
}

// IdempotencyStore guarda o resultado de operacoes ja aplicadas. Num adapter
// persistente, Remember precisa participar da mesma transacao do Save da
// carteira; senao um crash entre os dois reabre a janela de debito duplo.
type IdempotencyStore interface {
	Find(ctx context.Context, key string) (IdempotentResult, bool, error)
	Remember(ctx context.Context, key string, result IdempotentResult) error
}
