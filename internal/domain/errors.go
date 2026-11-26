package domain

import "errors"

// Erros de dominio. As camadas externas usam errors.Is para mapear cada um
// ao status HTTP correspondente.
var (
	ErrWalletNotFound     = errors.New("carteira nao encontrada")
	ErrInsufficientFunds  = errors.New("saldo insuficiente")
	ErrInvalidAmount      = errors.New("valor invalido: deve ser maior que zero")
	ErrInvalidOwner       = errors.New("nome do titular invalido")
	ErrInvalidPixKey      = errors.New("chave pix invalida")
	ErrPixKeyAlreadyInUse = errors.New("chave pix ja esta em uso")
	ErrSameWallet         = errors.New("transferencia para a mesma carteira")
	ErrBalanceOverflow    = errors.New("valor excede o saldo maximo representavel")
)
