package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
)

func main() {
	port := getenv("APP_PORT", "8080")
	mysqlDSN := getenv("MYSQL_DSN", "user:pass@tcp(mysql:3306)/appdb?parseTime=true")
	pgDSN := getenv("PG_DSN", "postgres://user:pass@postgres:5432/appdb?sslmode=disable")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			name = "Guest 😄"
		}
		fmt.Fprintf(w, "Welcome, %s\n", name)
	})

	http.HandleFunc("/health/mysql", func(w http.ResponseWriter, r *http.Request) {
		db, err := sql.Open("mysql", mysqlDSN)
		if err != nil {
			http.Error(w, "mysql open error: "+err.Error(), 500)
			return
		}
		defer db.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "mysql ping error: "+err.Error(), 500)
			return
		}
		w.Write([]byte("mysql ok"))
	})

	http.HandleFunc("/health/pg", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, pgDSN)
		if err != nil {
			http.Error(w, "pg connect error: "+err.Error(), 500)
			return
		}
		defer conn.Close(ctx)
		if err := conn.Ping(ctx); err != nil {
			http.Error(w, "pg ping error: "+err.Error(), 500)
			return
		}
		w.Write([]byte("pg ok"))
	})

	log.Printf("listening on :%s ...", port)
	http.ListenAndServe(":"+port, nil)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

