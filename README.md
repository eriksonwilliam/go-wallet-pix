# go-wallet-pix

Carteira digital com **PIX simulado**, escrita em **Go** usando apenas a
biblioteca padrão (net/http com roteamento por método do Go 1.22+, **zero
dependências externas**). Serve como template de um serviço Go limpo,
testável e com arquitetura em camadas.

Operações: abrir carteira, depositar, sacar e transferir via chave PIX, com
extrato de transações. Os valores são tratados em **centavos (int64)**,
evitando imprecisão de ponto flutuante em dinheiro.

## Destaques

- **Zero dependências** — só a stdlib. Sem `go.sum`, sem superfície de
  vulnerabilidade em terceiros.
- **Arquitetura em camadas** — domínio → aplicação → infraestrutura, com a
  dependência apontando sempre para dentro.
- **Invariante no domínio** — saldo nunca fica negativo; "valor positivo" é
  validado na borda. Separação clara entre regra de negócio e validação de
  entrada.
- **Cobertura de testes 100%** — domínio, casos de uso (com injeção de falhas
  de infraestrutura) e camada HTTP.
- **Imagem final distroless** — binário estático, sem shell, rodando como
  usuário não-root.

## Stack

- **Linguagem:** Go 1.24 (apenas stdlib)
- **HTTP:** `net/http` com roteamento por método (Go 1.22+)
- **Testes:** `testing` + `net/http/httptest`, cobertura 100%
- **Qualidade:** `gofmt` + `go vet`
- **Infra:** Docker multi-stage (build → distroless)

## Arquitetura

```
cmd/
└── api/                       # Bootstrap (composição das dependências)
internal/
├── domain/                    # Regras de negócio puras
│   ├── wallet.go              # Aggregate root + extrato
│   └── errors.go
├── application/               # Casos de uso
│   ├── service.go             # CreateWallet, Deposit, Withdraw, Transfer
│   └── ports.go               # WalletRepository (interface)
└── infrastructure/
    ├── memory/                # Repositório em memória (concorrente)
    └── httpapi/               # Handlers, rotas, DTOs e mapeamento de erros
```

## Como rodar

### Pré-requisitos

- Go 1.24+ (ou apenas Docker)

### Local

```bash
go run ./cmd/api
# a API sobe em http://localhost:3000 (configurável via PORT)
```

### Com Docker

```bash
docker compose up --build
```

## Endpoints

| Método | Rota                       | Descrição                            |
| ------ | -------------------------- | ------------------------------------ |
| GET    | `/health`                  | Health check                         |
| GET    | `/docs`                    | Swagger UI (documentacao interativa) |
| GET    | `/openapi.json`            | Especificacao OpenAPI 3               |
| POST   | `/wallets`                 | Abre uma carteira                    |
| GET    | `/wallets/{id}`            | Consulta carteira + extrato          |
| POST   | `/wallets/{id}/deposit`    | Deposita (valor em centavos)         |
| POST   | `/wallets/{id}/withdraw`   | Saca (valor em centavos)             |
| POST   | `/wallets/{id}/transfer`   | Transfere via chave PIX (simulado)   |

Exemplo de fluxo:

```bash
# abre duas carteiras
curl -X POST localhost:3000/wallets -d '{"owner":"Alice","pix_key":"alice@pix"}'
curl -X POST localhost:3000/wallets -d '{"owner":"Bob","pix_key":"bob@pix"}'

# deposita R$ 100,00 (10000 centavos) na carteira da Alice
curl -X POST localhost:3000/wallets/<id-alice>/deposit -d '{"amount_cents":10000}'

# transfere R$ 20,00 para o Bob via PIX
curl -X POST localhost:3000/wallets/<id-alice>/transfer \
  -d '{"to_pix_key":"bob@pix","amount_cents":2000}'
```

## Testes e cobertura

```bash
go test ./internal/... -cover
# relatório HTML:
go test ./internal/... -coverprofile=coverage.out && go tool cover -html=coverage.out
```

A cobertura é exigida em **100%** dos pacotes de `internal/` (a CI falha
abaixo disso). O `cmd/api` (bootstrap) fica fora da métrica.

## Licença

MIT.
