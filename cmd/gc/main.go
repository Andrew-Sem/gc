package main

import (
	"cmp"
	"log"
	"net/http"
	"os"

	"github.com/Andrew-Sem/gc/internal/web"
)

func main() {
	addr := cmp.Or(os.Getenv("GC_ADDR"), "localhost:8080")
	log.Printf("админка: http://%s/repositories", addr)
	log.Fatal(http.ListenAndServe(addr, web.New()))
}
