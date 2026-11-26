package httpapi

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// Metrics conta requisicoes HTTP no formato de texto do Prometheus, escrito a
// mao: um contador com tres labels nao justifica a dependencia do client oficial.
type Metrics struct {
	mu       sync.Mutex
	requests map[requestKey]uint64
}

type requestKey struct {
	method string
	route  string
	status int
}

func NewMetrics() *Metrics {
	return &Metrics{requests: make(map[requestKey]uint64)}
}

// Instrument envolve o roteador. O label de rota e o padrao registrado
// (/wallets/{id}), nunca o caminho real: um id por serie explodiria a
// cardinalidade no Prometheus.
func (m *Metrics) Instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		} else if _, path, found := strings.Cut(route, " "); found {
			route = path
		}
		m.mu.Lock()
		m.requests[requestKey{method: r.Method, route: route, status: recorder.status}]++
		m.mu.Unlock()
	})
}

// ServeHTTP expoe /metrics.
func (m *Metrics) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	m.mu.Lock()
	counts := make(map[requestKey]uint64, len(m.requests))
	keys := make([]requestKey, 0, len(m.requests))
	for key, count := range m.requests {
		counts[key] = count
		keys = append(keys, key)
	}
	m.mu.Unlock()

	// Ordem estavel: a saida fica comparavel entre scrapes e testavel.
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.route != b.route {
			return a.route < b.route
		}
		if a.method != b.method {
			return a.method < b.method
		}
		return a.status < b.status
	})
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	var body strings.Builder
	body.WriteString("# HELP http_requests_total Requisicoes HTTP por metodo, rota e status.\n")
	body.WriteString("# TYPE http_requests_total counter\n")
	for _, key := range keys {
		fmt.Fprintf(&body, "http_requests_total{method=%q,route=%q,status=\"%d\"} %d\n",
			key.method, key.route, key.status, counts[key])
	}
	_, _ = w.Write([]byte(body.String()))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
