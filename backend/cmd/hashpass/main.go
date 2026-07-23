// Comando auxiliar para generar el hash bcrypt de una contraseña.
//
// Uso:
//
//	go run ./cmd/hashpass -p "MiContrasena123!"
//
// Pegue el hash resultante en `db/migrations/0002_seed.up.sql`.
package main

import (
	"flag"
	"fmt"
	"os"

	"gitlab.com/syncbox/backend/internal/crypto"
)

func main() {
	pwd := flag.String("p", "", "contraseña en texto plano")
	flag.Parse()
	if *pwd == "" {
		fmt.Fprintln(os.Stderr, "uso: hashpass -p <contraseña>")
		os.Exit(2)
	}
	h, err := crypto.HashPassword(*pwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(h)
}
