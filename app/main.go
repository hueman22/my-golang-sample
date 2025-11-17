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

	mysqlrepo "example.com/my-golang-sample/app/internal/infra/persistence/mysql"
	userhttp "example.com/my-golang-sample/app/internal/interface/http"
	useruc "example.com/my-golang-sample/app/internal/usecase/user"
)

func main() {
	port := getenv("APP_PORT", "8080")
	mysqlDSN := getenv("MYSQL_DSN", "user:pass@tcp(mysql:3306)/appdb?parseTime=true&charset=utf8mb4&loc=Local")
	pgDSN := getenv("PG_DSN", "postgres://user:pass@postgres:5432/appdb?sslmode=disable")

	// 1) Connect MySQL
	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		log.Fatalf("mysql open error: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("mysql ping error: %v", err)
	}
	log.Println("MySQL connected")

	// 2) Ensure tables
	if err := ensureTables(db); err != nil {
		log.Fatalf("ensureTables error: %v", err)
	}

	// 3) Init DDD stack for user
	userRepo := mysqlrepo.NewUserRepository(db)
	userSvc := useruc.NewService(userRepo)
	userHandler := userhttp.NewUserHandler(userSvc)

	// 4) Setup mux & routes
	mux := http.NewServeMux()

	// Root hello
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			name = "Guest 😄"
		}
		fmt.Fprintf(w, "Welcome, %s\n", name)
	})

	// Health MySQL
	mux.HandleFunc("/health/mysql", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "mysql ping error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write([]byte("mysql ok"))
	})

	// Health Postgres
	mux.HandleFunc("/health/pg", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, pgDSN)
		if err != nil {
			http.Error(w, "pg connect error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer conn.Close(ctx)
		if err := conn.Ping(ctx); err != nil {
			http.Error(w, "pg ping error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write([]byte("pg ok"))
	})

	// Users API (POST /users với action=create|get|update|delete)
	mux.Handle("/users", userHandler)

	log.Printf("listening on :%s ...", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func ensureTables(db *sql.DB) error {
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS user_roles (
            id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
            code VARCHAR(64) NOT NULL UNIQUE,
            name VARCHAR(255) NOT NULL,
            description TEXT NULL,
            is_system TINYINT(1) NOT NULL DEFAULT 0,
            created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
        );
    `)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
            name VARCHAR(255) NOT NULL,
            email VARCHAR(255) NOT NULL UNIQUE,
            password_hash VARCHAR(255) NOT NULL,
            user_role_id BIGINT UNSIGNED NOT NULL,
            created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            CONSTRAINT fk_users_user_role_id
                FOREIGN KEY (user_role_id) REFERENCES user_roles(id)
        );
    `)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
        INSERT IGNORE INTO user_roles (code, name, description, is_system)
        VALUES 
          ('SUPER_ADMIN', 'Super Admin', 'Highest system administrator', 1),
          ('ADMIN', 'Admin', 'System administrator', 1),
          ('CUSTOMER', 'Customer', 'Normal customer user', 1);
    `)
	if err != nil {
		return err
	}

	return nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
