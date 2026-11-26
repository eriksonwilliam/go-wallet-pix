package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eriksonwilliam/go-wallet-pix/internal/application"
	"github.com/eriksonwilliam/go-wallet-pix/internal/domain"
	"github.com/eriksonwilliam/go-wallet-pix/internal/infrastructure/httpapi"
	"github.com/eriksonwilliam/go-wallet-pix/internal/infrastructure/memory"
)

func newRouter() http.Handler {
	repo := memory.New()
	n := 0
	idGen := func() string { n++; return "wallet-" + string(rune('a'+n-1)) }
	clock := func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	svc := application.NewWalletService(repo, memory.NewIdempotencyStore(), idGen, clock)
	return httpapi.NewRouter(httpapi.NewHandler(svc), httpapi.NewMetrics())
}

func do(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return doWithHeaders(t, router, method, path, body, nil)
}

func doWithHeaders(t *testing.T, router http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeWallet(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("json invalido: %v (%s)", err, rec.Body.String())
	}
	return out
}

func TestHealth(t *testing.T) {
	rec := do(t, newRouter(), http.MethodGet, "/health", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestOpenAPISpec(t *testing.T) {
	rec := do(t, newRouter(), http.MethodGet, "/openapi.json", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "\"openapi\"") {
		t.Fatalf("spec sem campo openapi: %s", rec.Body.String())
	}
}

func TestDocsUI(t *testing.T) {
	rec := do(t, newRouter(), http.MethodGet, "/docs", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "swagger-ui") {
		t.Fatalf("html sem swagger-ui")
	}
}

func TestCreateWallet(t *testing.T) {
	router := newRouter()

	rec := do(t, router, http.MethodPost, "/wallets", `{"owner":"Alice","pix_key":"alice@pix"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := decodeWallet(t, rec)
	if body["owner"] != "Alice" || body["balance"] != "R$ 0,00" {
		t.Fatalf("resposta inesperada: %v", body)
	}

	// corpo invalido
	if rec := do(t, router, http.MethodPost, "/wallets", `{`); rec.Code != http.StatusBadRequest {
		t.Fatalf("esperava 400 para corpo invalido, obtive %d", rec.Code)
	}

	// titular invalido -> 400
	if rec := do(t, router, http.MethodPost, "/wallets", `{"owner":"","pix_key":"x@pix"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("esperava 400 para titular invalido, obtive %d", rec.Code)
	}

	// chave pix duplicada -> 409
	if rec := do(t, router, http.MethodPost, "/wallets", `{"owner":"Outro","pix_key":"alice@pix"}`); rec.Code != http.StatusConflict {
		t.Fatalf("esperava 409 para chave duplicada, obtive %d", rec.Code)
	}
}

func TestWalletLifecycle(t *testing.T) {
	router := newRouter()

	alice := decodeWallet(t, do(t, router, http.MethodPost, "/wallets", `{"owner":"Alice","pix_key":"alice@pix"}`))
	bob := decodeWallet(t, do(t, router, http.MethodPost, "/wallets", `{"owner":"Bob","pix_key":"bob@pix"}`))
	aliceID := alice["id"].(string)

	// deposito
	rec := do(t, router, http.MethodPost, "/wallets/"+aliceID+"/deposit", `{"amount_cents":10000}`)
	if rec.Code != http.StatusOK || decodeWallet(t, rec)["balance_cents"].(float64) != 10000 {
		t.Fatalf("deposito falhou: %d %s", rec.Code, rec.Body.String())
	}

	// saque
	rec = do(t, router, http.MethodPost, "/wallets/"+aliceID+"/withdraw", `{"amount_cents":3000}`)
	if rec.Code != http.StatusOK || decodeWallet(t, rec)["balance_cents"].(float64) != 7000 {
		t.Fatalf("saque falhou: %d %s", rec.Code, rec.Body.String())
	}

	// transferencia PIX para bob
	rec = do(t, router, http.MethodPost, "/wallets/"+aliceID+"/transfer", `{"to_pix_key":"bob@pix","amount_cents":2000}`)
	if rec.Code != http.StatusOK || decodeWallet(t, rec)["balance_cents"].(float64) != 5000 {
		t.Fatalf("transferencia falhou: %d %s", rec.Code, rec.Body.String())
	}

	// consulta bob refletindo a transferencia
	bobID := bob["id"].(string)
	rec = do(t, router, http.MethodGet, "/wallets/"+bobID, "")
	if rec.Code != http.StatusOK || decodeWallet(t, rec)["balance_cents"].(float64) != 2000 {
		t.Fatalf("consulta bob falhou: %d %s", rec.Code, rec.Body.String())
	}
}

func TestErrorMappings(t *testing.T) {
	router := newRouter()
	alice := decodeWallet(t, do(t, router, http.MethodPost, "/wallets", `{"owner":"Alice","pix_key":"alice@pix"}`))
	aliceID := alice["id"].(string)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		status int
	}{
		{"consulta inexistente", http.MethodGet, "/wallets/xyz", "", http.StatusNotFound},
		{"deposito corpo invalido", http.MethodPost, "/wallets/" + aliceID + "/deposit", `{`, http.StatusBadRequest},
		{"deposito valor invalido", http.MethodPost, "/wallets/" + aliceID + "/deposit", `{"amount_cents":0}`, http.StatusBadRequest},
		{"saque corpo invalido", http.MethodPost, "/wallets/" + aliceID + "/withdraw", `{`, http.StatusBadRequest},
		{"saque insuficiente", http.MethodPost, "/wallets/" + aliceID + "/withdraw", `{"amount_cents":999999}`, http.StatusConflict},
		{"transferencia corpo invalido", http.MethodPost, "/wallets/" + aliceID + "/transfer", `{`, http.StatusBadRequest},
		{"transferencia mesma carteira", http.MethodPost, "/wallets/" + aliceID + "/transfer", `{"to_pix_key":"alice@pix","amount_cents":10}`, http.StatusBadRequest},
		{"transferencia destino inexistente", http.MethodPost, "/wallets/" + aliceID + "/transfer", `{"to_pix_key":"nao@existe","amount_cents":10}`, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, router, tc.method, tc.path, tc.body)
			if rec.Code != tc.status {
				t.Fatalf("esperava %d, obtive %d (%s)", tc.status, rec.Code, rec.Body.String())
			}
		})
	}
}

// falhaRepo retorna sempre um erro generico, para exercitar o mapeamento 500.
type falhaRepo struct{}

func (falhaRepo) Save(context.Context, ...*domain.Wallet) error { return errors.New("indisponivel") }
func (falhaRepo) FindByID(context.Context, string) (*domain.Wallet, error) {
	return nil, errors.New("indisponivel")
}
func (falhaRepo) FindByPixKey(context.Context, string) (*domain.Wallet, error) {
	return nil, errors.New("indisponivel")
}

func TestInternalServerError(t *testing.T) {
	svc := application.NewWalletService(falhaRepo{}, memory.NewIdempotencyStore(), func() string { return "x" }, time.Now)
	router := httpapi.NewRouter(httpapi.NewHandler(svc), httpapi.NewMetrics())

	rec := do(t, router, http.MethodGet, "/wallets/qualquer", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("esperava 500, obtive %d", rec.Code)
	}
}

func TestIdempotencyKeyHeader(t *testing.T) {
	router := newRouter()
	alice := decodeWallet(t, do(t, router, http.MethodPost, "/wallets", `{"owner":"Alice","pix_key":"alice@pix"}`))
	path := "/wallets/" + alice["id"].(string) + "/deposit"
	key := map[string]string{"Idempotency-Key": "dep-1"}

	for attempt := 0; attempt < 3; attempt++ {
		rec := doWithHeaders(t, router, http.MethodPost, path, `{"amount_cents":1000}`, key)
		if rec.Code != http.StatusOK || decodeWallet(t, rec)["balance_cents"].(float64) != 1000 {
			t.Fatalf("tentativa %d: %d %s", attempt, rec.Code, rec.Body.String())
		}
	}

	conflict := doWithHeaders(t, router, http.MethodPost, path, `{"amount_cents":2000}`, key)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("chave reutilizada com outro valor: esperava 409, obtive %d", conflict.Code)
	}
}

func TestOverflowMapsToConflict(t *testing.T) {
	router := newRouter()
	alice := decodeWallet(t, do(t, router, http.MethodPost, "/wallets", `{"owner":"Alice","pix_key":"alice@pix"}`))
	path := "/wallets/" + alice["id"].(string) + "/deposit"
	do(t, router, http.MethodPost, path, `{"amount_cents":9223372036854775807}`)

	if rec := do(t, router, http.MethodPost, path, `{"amount_cents":1}`); rec.Code != http.StatusConflict {
		t.Fatalf("esperava 409, obtive %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestMetricsCountRequestsByRoutePattern(t *testing.T) {
	router := newRouter()
	alice := decodeWallet(t, do(t, router, http.MethodPost, "/wallets", `{"owner":"Alice","pix_key":"alice@pix"}`))
	do(t, router, http.MethodGet, "/wallets/"+alice["id"].(string), "")
	do(t, router, http.MethodGet, "/wallets/nao-existe", "")
	do(t, router, http.MethodGet, "/rota/inexistente", "")
	do(t, router, http.MethodPost, "/outra/inexistente", "")

	rec := do(t, router, http.MethodGet, "/metrics", "")
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("status %d, content-type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, want := range []string{
		`http_requests_total{method="POST",route="/wallets",status="201"} 1`,
		`http_requests_total{method="GET",route="/wallets/{id}",status="200"} 1`,
		`http_requests_total{method="GET",route="/wallets/{id}",status="404"} 1`,
		`http_requests_total{method="GET",route="unmatched",status="404"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("faltou %q em:\n%s", want, body)
		}
	}
	if strings.Contains(body, alice["id"].(string)) {
		t.Fatal("id da carteira vazou para o label de rota")
	}
}
