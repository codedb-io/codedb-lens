// waitdb blocks until the CI database accepts connections (up to 3 minutes). Usage: waitdb mssql|pg
// SQL Server 2017 self-signed certificates have negative serial numbers, which Go rejects by default.
//
//go:debug x509negativeserial=1
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/codedb-io/codedb-lens/internal/adapter"
)

func main() {
	c := adapter.Conn{Engine: os.Args[1], Server: os.Getenv("LENS_MSSQL_SERVER"), User: "sa",
		Password: os.Getenv("MSSQL_SA_PASSWORD"), TrustServerCert: true, DSN: os.Getenv("LENS_PG_DSN")}
	if c.Engine == "mssql" {
		c.DSN = ""
	}
	var err error
	for deadline := time.Now().Add(3 * time.Minute); time.Now().Before(deadline); time.Sleep(3 * time.Second) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var s adapter.Session
		if s, err = adapter.Open(ctx, c); err == nil {
			s.Close()
			cancel()
			fmt.Println("database ready")
			return
		}
		cancel()
	}
	fmt.Fprintln(os.Stderr, "database not ready:", err)
	os.Exit(1)
}
