package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/eriksonwilliam/go-wallet-pix/internal/application"
	"github.com/eriksonwilliam/go-wallet-pix/internal/infrastructure/httpapi"
	"github.com/eriksonwilliam/go-wallet-pix/internal/infrastructure/memory"
)

func main() {
	repo := memory.New()
	service := application.NewWalletService(repo, newUUID, time.Now)
	handler := httpapi.NewHandler(service)
	router := httpapi.NewRouter(handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("go-wallet-pix ouvindo em :%s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatal(err)
	}
}

// newUUID gera um UUID v4 usando apenas a biblioteca padrao.
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
