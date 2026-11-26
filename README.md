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
- **Idempotência** — `Idempotency-Key` em depósito, saque e transferência:
  um retry do cliente devolve o resultado original sem debitar de novo.
- **Transferência tudo-ou-nada** — o domínio valida as duas pontas antes de
  alterar qualquer uma, e o repositório grava origem e destino juntas.
- **Sem corrida de dados** — `go test -race` na CI, com teste de propriedade:
  transferências concorrentes nos dois sentidos conservam o total.
- **`/metrics` Prometheus** — contador por método, padrão de rota e status,
  escrito à mão (sem client oficial).
- **Imagem final distroless** — binário estático, sem shell, rodando como
  usuário não-root.

## Stack

- **Linguagem:** Go (toolchain 1.27 no build e na CI; `go.mod` exige 1.24+), apenas stdlib
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
│   └── ports.go               # WalletRepository, IdempotencyStore
└── infrastructure/
    ├── memory/                # Repositório e registro de idempotência em memória
    └── httpapi/               # Handlers, rotas, DTOs, erros e /metrics
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
| GET    | `/metrics`                 | Métricas no formato Prometheus       |
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
abaixo disso). O `cmd/api` (bootstrap) fica fora da métrica. A CI também roda
`go test -race ./internal/...`.

## Defeitos corrigidos em 26/11/2026

Encontrados numa auditoria do código e reproduzidos com teste antes da correção:

- **Corrida de dados.** O repositório em memória devolvia o ponteiro que
  guardava; o handler serializava a carteira enquanto outra requisição a
  alterava. `go test -race` apontava 3 corridas. Agora o repositório guarda e
  devolve cópias.
- **Overflow de saldo.** O depósito só validava `amount > 0`; `MaxInt64 + 1`
  deixava o saldo em `-9223372036854775808`. Agora é `ErrBalanceOverflow` (409).

## Limitações

- **PIX é simulado**: transferência entre carteiras do próprio serviço, sem
  SPI/DICT nem integração com o Banco Central.
- **Persistência em memória**: tudo se perde ao reiniciar, e o registro de
  idempotência não expira chaves. Um adapter persistente precisa gravar
  carteira e idempotência na mesma transação.
- **Uma instância**: a serialização das escritas é um mutex do processo; com
  mais de uma réplica ela teria de vir do banco (lock de linha).

## Licença

MIT.
