package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"

	mysqlrepo "example.com/my-golang-sample/app/internal/infra/persistence/mysql"
	"example.com/my-golang-sample/app/internal/infra/security"
	apihttp "example.com/my-golang-sample/app/internal/interface/http"
	authuc "example.com/my-golang-sample/app/internal/usecase/auth"
	cartuc "example.com/my-golang-sample/app/internal/usecase/cart"
	categoryuc "example.com/my-golang-sample/app/internal/usecase/category"
	orderuc "example.com/my-golang-sample/app/internal/usecase/order"
	productuc "example.com/my-golang-sample/app/internal/usecase/product"
	useruc "example.com/my-golang-sample/app/internal/usecase/user"
	userroleuc "example.com/my-golang-sample/app/internal/usecase/userrole"
)

func main() {
	port := getenv("APP_PORT", "8080")
	mysqlDSN := getenv("MYSQL_DSN", "user:pass@tcp(mysql:3306)/appdb?parseTime=true&charset=utf8mb4&loc=Local")
	pgDSN := getenv("PG_DSN", "postgres://user:pass@postgres:5432/appdb?sslmode=disable")
	jwtSecret := getenv("JWT_SECRET", "change-me")

	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		log.Fatalf("mysql open error: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("mysql ping error: %v", err)
	}
	log.Println("MySQL connected")

	if err := ensureTables(db); err != nil {
		log.Fatalf("ensureTables error: %v", err)
	}

	passwordSvc := security.NewBcryptService(0)
	tokenSvc := security.NewJWTService(jwtSecret, 24*time.Hour)

	userRepo := mysqlrepo.NewUserRepository(db)
	roleRepo := mysqlrepo.NewUserRoleRepository(db)
	categoryRepo := mysqlrepo.NewCategoryRepository(db)
	productRepo := mysqlrepo.NewProductRepository(db)
	cartRepo := mysqlrepo.NewCartRepository(db)
	orderRepo := mysqlrepo.NewOrderRepository(db)

	userSvc := useruc.NewService(userRepo, passwordSvc)
	roleSvc := userroleuc.NewService(roleRepo)
	categorySvc := categoryuc.NewService(categoryRepo)
	productSvc := productuc.NewService(productRepo)
	orderSvc := orderuc.NewService(orderRepo)
	cartSvc := cartuc.NewService(cartRepo, productRepo, orderRepo)
	authSvc := authuc.NewService(userRepo, passwordSvc, tokenSvc)

	if err := seedSuperAdmin(db, passwordSvc, getenv("SUPER_ADMIN_EMAIL", ""), getenv("SUPER_ADMIN_PASSWORD", "")); err != nil {
		log.Printf("seed super admin error: %v", err)
	}

	api := apihttp.NewAPI(apihttp.Dependencies{
		AuthService:     authSvc,
		UserService:     userSvc,
		UserRoleService: roleSvc,
		CategoryService: categorySvc,
		ProductService:  productSvc,
		CartService:     cartSvc,
		OrderService:    orderSvc,
		TokenService:    tokenSvc,
	})

	router := api.Router()

	// 👇 THÊM ĐOẠN NÀY
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		name := r.URL.Query().Get("name")
		if name == "" {
			name = "Guest"
		}

		msg := fmt.Sprintf("Welcome %s to My Golang Ecommerce API", name)
		w.Write([]byte(fmt.Sprintf(`{"message": %q, "status": "ok"}`, msg)))
	})

	// 👆 THÊM ĐOẠN NÀY

	router.Get("/health/mysql", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "mysql ping error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write([]byte("mysql ok"))
	})

	router.Get("/health/postgres", func(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("listening on :%s ...", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatal(err)
	}
}

func ensureTables(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS user_roles (
            id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
            code VARCHAR(64) NOT NULL UNIQUE,
            name VARCHAR(255) NOT NULL,
            description TEXT NULL,
            is_system TINYINT(1) NOT NULL DEFAULT 0,
            created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS users (
            id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
            name VARCHAR(255) NOT NULL,
            email VARCHAR(255) NOT NULL UNIQUE,
            password_hash VARCHAR(255) NOT NULL,
            user_role_id BIGINT UNSIGNED NOT NULL,
            created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            CONSTRAINT fk_users_user_role_id
                FOREIGN KEY (user_role_id) REFERENCES user_roles(id)
        );`,
		`CREATE TABLE IF NOT EXISTS categories (
            id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
            name VARCHAR(255) NOT NULL,
            slug VARCHAR(255) NOT NULL UNIQUE,
            description TEXT NULL,
            is_active TINYINT(1) NOT NULL DEFAULT 1,
            created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS products (
            id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
            name VARCHAR(255) NOT NULL,
            description TEXT NULL,
            price DECIMAL(12,2) NOT NULL,
            stock BIGINT NOT NULL DEFAULT 0,
            category_id BIGINT UNSIGNED NOT NULL,
            is_active TINYINT(1) NOT NULL DEFAULT 1,
            created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            CONSTRAINT fk_products_category_id FOREIGN KEY (category_id) REFERENCES categories(id)
        );`,
		`CREATE TABLE IF NOT EXISTS cart_items (
            id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
            user_id BIGINT UNSIGNED NOT NULL,
            product_id BIGINT UNSIGNED NOT NULL,
            quantity BIGINT NOT NULL,
            created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            UNIQUE KEY uniq_cart_user_product (user_id, product_id),
            CONSTRAINT fk_cart_user_id FOREIGN KEY (user_id) REFERENCES users(id),
            CONSTRAINT fk_cart_product_id FOREIGN KEY (product_id) REFERENCES products(id)
        );`,
		`CREATE TABLE IF NOT EXISTS orders (
            id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
            user_id BIGINT UNSIGNED NOT NULL,
            status VARCHAR(32) NOT NULL,
            payment_method VARCHAR(32) NOT NULL,
            total_amount DECIMAL(14,2) NOT NULL,
            created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            CONSTRAINT fk_orders_user_id FOREIGN KEY (user_id) REFERENCES users(id)
        );`,
		`CREATE TABLE IF NOT EXISTS order_items (
            id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
            order_id BIGINT UNSIGNED NOT NULL,
            product_id BIGINT UNSIGNED NOT NULL,
            product_name VARCHAR(255) NOT NULL,
            unit_price DECIMAL(12,2) NOT NULL,
            quantity BIGINT NOT NULL,
            created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            CONSTRAINT fk_order_items_order_id FOREIGN KEY (order_id) REFERENCES orders(id),
            CONSTRAINT fk_order_items_product_id FOREIGN KEY (product_id) REFERENCES products(id)
        );`,
		`INSERT IGNORE INTO user_roles (code, name, description, is_system)
        VALUES 
          ('SUPER_ADMIN', 'Super Admin', 'Highest system administrator', 1),
          ('ADMIN', 'Admin', 'System administrator', 1),
          ('CUSTOMER', 'Customer', 'Normal customer user', 1);`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	if err := ensureCategorySlug(db); err != nil {
		return err
	}

	return nil
}

func ensureCategorySlug(db *sql.DB) error {
	if _, err := db.Exec(`ALTER TABLE categories ADD COLUMN slug VARCHAR(255) NOT NULL DEFAULT '' AFTER name`); err != nil {
		if !isDuplicateColumnErr(err) {
			return err
		}
	}

	if _, err := db.Exec(`UPDATE categories SET slug = LOWER(REPLACE(name, ' ', '-')) WHERE slug IS NULL OR slug = ''`); err != nil {
		return err
	}

	if _, err := db.Exec(`ALTER TABLE categories ADD UNIQUE KEY uniq_categories_slug (slug)`); err != nil {
		if !isDuplicateKeyErr(err) {
			return err
		}
	}

	if _, err := db.Exec(`ALTER TABLE categories MODIFY COLUMN slug VARCHAR(255) NOT NULL`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "unknown column") {
			return err
		}
	}

	return nil
}

func isDuplicateColumnErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
}

func isDuplicateKeyErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate key name")
}

func seedSuperAdmin(db *sql.DB, hasher interface{ Hash(string) (string, error) }, email, password string) error {
	if email == "" || password == "" {
		return nil
	}

	var exists int
	if err := db.QueryRow(`SELECT COUNT(1) FROM users WHERE email = ?`, email).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}

	var roleID int64
	if err := db.QueryRow(`SELECT id FROM user_roles WHERE code = 'SUPER_ADMIN'`).Scan(&roleID); err != nil {
		return err
	}

	hash, err := hasher.Hash(password)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
        INSERT INTO users (name, email, password_hash, user_role_id)
        VALUES (?, ?, ?, ?)`,
		"Seed Super Admin", email, hash, roleID,
	)
	return err
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
