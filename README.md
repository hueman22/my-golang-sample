# Ecommerce Backend API

REST API backend for a simple ecommerce application built with Go (Golang) and MySQL, following Domain-Driven Design (DDD).
The project implements authentication, role-based access control, admin management features, and customer shopping flows
from product browsing to cart and checkout.

## Overview

This project provides:

- **DDD architecture** (domain → usecase → infra → http)
- **JWT authentication** and **role-based authorization**
- **Admin features** to manage user roles, users, categories, products, and orders
- **Customer features** to browse products, manage a cart, and checkout (COD/TAMARA)
- **Unit tests** for usecases and **HTTP feature tests** for handlers

Only features that are actually implemented in this repository (and defined in `app/EPIC_ecommerce_backend.md`) are documented.

## Features

- **Auth Login**
  - `POST /api/v1/auth/login` with email and password
  - Returns a JWT token containing user ID and role information

- **User Roles**
  - Admin CRUD for user roles
  - Role codes are validated and fixed via the domain rules
  - Used for access control across the system

- **Users**
  - Admin CRUD for users
  - Email must be unique
  - Role assignment rules:
    - **ADMIN is NOT allowed to create or update users with `role_code = ADMIN`**
    - **ONLY SUPER_ADMIN can create or update users with `role_code = ADMIN`**

- **Categories**
  - Admin CRUD for product categories

- **Products**
  - Admin CRUD for products
  - Public product browsing (guest and customer)

- **Cart**
  - Authenticated customers can add products to their cart
  - View current cart contents

- **Checkout**
  - Authenticated customers can checkout their cart
  - Supported payment methods: `COD`, `TAMARA`
  - Creates orders and order_items from the cart and clears the cart on success

- **Orders (Admin)**
  - Admin can list all orders
  - Admin can view the details of an order
  - Admin can update the status of an order

- **Access Control Rules**
  - All `/api/v1/admin/*` endpoints require a valid JWT and role `ADMIN` or `SUPER_ADMIN`
  - Customers and guests cannot call admin endpoints

## Architecture (DDD)

The project is organized into clear layers:

```text
app/
├── main.go                         # Entrypoint, DI wiring, DB setup, HTTP server
├── internal/
│   ├── domain/                     # Domain models and domain errors
│   │   ├── user/                   # User entity, RoleCode, policies
│   │   ├── userrole/               # UserRole domain
│   │   ├── category/               # Category domain
│   │   ├── product/                # Product domain
│   │   ├── cart/                   # Cart domain
│   │   └── order/                  # Order domain
│   ├── usecase/                    # Application services (business rules)
│   │   ├── auth/                   # Login
│   │   ├── user/                   # Users
│   │   ├── userrole/               # User roles
│   │   ├── category/               # Categories
│   │   ├── product/                # Products
│   │   ├── cart/                   # Cart
│   │   ├── checkout/               # Checkout
│   │   └── order/                  # Orders
│   ├── infra/
│   │   ├── persistence/mysql/      # MySQL repositories
│   │   └── security/               # JWT + password hashing
│   └── interface/http/             # HTTP layer (chi router, handlers, middleware)
│       ├── api.go                  # Router and route registration
│       ├── middleware.go           # Auth middleware and role enforcement
│       ├── auth_handlers.go        # Login
│       ├── admin_handlers.go       # Admin (roles, users, categories, products, orders)
│       ├── product_handlers.go     # Public product browsing
│       └── cart_handlers.go        # Cart + checkout
```

### Request Flow

```text
HTTP Request
  → HTTP Handler (decode JSON, basic validation)
  → Usecase Service (business rules)
  → Repository Interface
  → MySQL Repository Implementation
  → MySQL Database
```

## Getting Started

### Prerequisites

- Go 1.21+ (for local development)
- Docker and Docker Compose (for MySQL and/or running the app in a container)

### Local Run

From the repository root:

```bash
git clone <repository-url>
cd my-golang-sample
```

1. **Start MySQL via Docker**

```bash
docker compose -f docker-compose.app.yml up -d mysql
```

2. **Configure environment**

```bash
cd app
cp env.example .env
# Edit .env as needed (MYSQL_DSN, APP_PORT, JWT_SECRET, SUPER_ADMIN_*).
export $(grep -v '^#' .env | xargs)
```

3. **Run the application**

```bash
go run ./...
```

The app listens on `APP_PORT` (from `.env`). When using Docker (see below), port `20000` on the host maps to the app port in the container.

## Docker Run

To run the app and database using Docker:

```bash
docker compose -f docker-compose.app.yml up -d mysql
docker compose -f docker-compose.app.yml up --build app
```

- App: `http://localhost:20000`
- MySQL: `localhost:13306` (user: `user`, password: `pass`, DB: `appdb`)

## Database Seed

On startup, `main.go`:

1. Ensures core tables exist:
   - `user_roles`, `users`, `categories`, `products`, `cart_items`, `orders`, `order_items`
2. Inserts default roles into `user_roles`:
   - `SUPER_ADMIN`, `ADMIN`, `CUSTOMER`
3. Seeds a `SUPER_ADMIN` user if:
   - `SUPER_ADMIN_EMAIL` and `SUPER_ADMIN_PASSWORD` are provided, and
   - no user with that email already exists.

No manual migrations are required for this sample; table creation and basic seeding are done in code.

## API Routes Summary

Only implemented routes are listed here.

### Root Endpoint (Important)

| Method | Endpoint | Description              |
|--------|----------|--------------------------|
| `GET`  | `/`      | Welcome JSON message     |

The root endpoint `"/"` is implemented in `main.go` and **must not be modified**.

### Auth

| Method | Endpoint               | Description                  |
|--------|------------------------|------------------------------|
| `POST` | `/api/v1/auth/login`   | Login, returns JWT token     |

### Public Product Browsing

| Method | Endpoint                    | Description               |
|--------|-----------------------------|---------------------------|
| `GET`  | `/api/v1/products`          | List products             |
| `GET`  | `/api/v1/products/{id}`     | Get product by ID         |

### Customer (Authenticated)

Requires `Authorization: Bearer <token>` and a logged-in user.

| Method | Endpoint                    | Description                  |
|--------|-----------------------------|------------------------------|
| `GET`  | `/api/v1/me/cart`           | Get current user cart        |
| `POST` | `/api/v1/me/cart/items`     | Add item to cart             |
| `POST` | `/api/v1/me/checkout`       | Checkout cart (COD/TAMARA)   |

### Admin (ADMIN or SUPER_ADMIN)

All admin endpoints are prefixed with `/api/v1/admin` and require a valid JWT with role `ADMIN` or `SUPER_ADMIN`.

**User Roles**

- `GET  /api/v1/admin/user-roles`
- `POST /api/v1/admin/user-roles`
- `GET  /api/v1/admin/user-roles/{id}`
- `PUT  /api/v1/admin/user-roles/{id}`
- `DELETE /api/v1/admin/user-roles/{id}`

**Users**

- `GET  /api/v1/admin/users`
- `POST /api/v1/admin/users`
- `GET  /api/v1/admin/users/{id}`
- `PUT  /api/v1/admin/users/{id}`
- `DELETE /api/v1/admin/users/{id}`

**Categories**

- `GET  /api/v1/admin/categories`
- `POST /api/v1/admin/categories`
- `GET  /api/v1/admin/categories/{id}`
- `PUT  /api/v1/admin/categories/{id}`
- `DELETE /api/v1/admin/categories/{id}`

**Products**

- `GET  /api/v1/admin/products`
- `POST /api/v1/admin/products`
- `PUT  /api/v1/admin/products/{id}`
- `DELETE /api/v1/admin/products/{id}`

**Orders**

- `GET   /api/v1/admin/orders`
- `GET   /api/v1/admin/orders/{id}`
- `PATCH /api/v1/admin/orders/{id}` (update status)

## Testing Guide (Unit + Feature)

From the `app` directory:

```bash
cd app
go test ./... -v
```

### Unit Tests (Usecase Layer)

- Located in `app/internal/usecase/*/*_test.go`
- Use fake/in-memory repositories (no real DB)
- Cover business rules for:
  - Auth
  - User roles
  - Users
  - Categories
  - Products
  - Cart
  - Checkout
  - Orders

Examples:

```bash
go test ./internal/usecase/user -v
go test ./internal/usecase/cart -v
go test ./internal/usecase/checkout -v
```

### HTTP Feature Tests

- Located in `app/internal/interface/http/*_handler_test.go`
- Use `httptest` and the real router
- Cover:
  - Login
  - Admin users & roles
  - Products (guest + admin)
  - Cart + checkout flows
  - Admin orders

Example:

```bash
go test ./internal/interface/http -v
```

## Smoke Tests

After running the app (locally or via Docker), you can do quick checks:

### 1. Root

```bash
curl http://localhost:20000/
curl http://localhost:20000/?name=Developer
```

### 2. Login

```bash
curl -X POST http://localhost:20000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"super.admin@example.com","password":"ChangeMe123!"}'
```

### 3. Products

```bash
curl http://localhost:20000/api/v1/products
```

### 4. Admin Users (Requires TOKEN from login)

```bash
export TOKEN=your-jwt-token

curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:20000/api/v1/admin/users
```

### 5. Cart & Checkout (Customer)

```bash
# Add item to cart
curl -X POST http://localhost:20000/api/v1/me/cart/items \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"product_id":1,"quantity":1}'

# View cart
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:20000/api/v1/me/cart

# Checkout
curl -X POST http://localhost:20000/api/v1/me/checkout \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"payment_method":"COD"}'
```

## Commands

### Development

```bash
cd app

# Run app
go run ./...

# Run all tests
go test ./... -v
```

### Docker

```bash
# Start MySQL
docker compose -f docker-compose.app.yml up -d mysql

# Start app
docker compose -f docker-compose.app.yml up --build app

# Stop everything
docker compose -f docker-compose.app.yml down
```

## Security

### Authentication

- Login via `POST /api/v1/auth/login`
- JWT contains user ID and role code
- JWT required for:
  - `/api/v1/me/*` (customer features)
  - `/api/v1/admin/*` (admin features)

### Access Control Rules

- **SUPER_ADMIN**
  - Full access to admin APIs
  - Can create/update users with any role (including ADMIN)
- **ADMIN**
  - Access to admin APIs, but:
  - **Cannot create or update users with `role_code = ADMIN`**
- **CUSTOMER**
  - Can browse products, manage cart, and checkout
  - Cannot access admin routes
- **GUEST**
  - Can browse products and access the root endpoint

If the role assignment rule is violated (e.g. ADMIN tries to create another ADMIN),
the usecase returns an error and **no change is persisted**.

## Contribution Rules

- Keep business logic in `internal/usecase/*`
- Keep HTTP handlers thin (decode → validate → call usecase)
- Add or update unit tests when changing usecases
- Add or update HTTP feature tests when changing handlers or routes
- Ensure `go test ./...` passes before committing

**Important:**  
Do **not** modify the root `"/"` endpoint in `main.go`. It is required by the project and must remain stable.

## License

Add your license information here (for example: MIT, Apache 2.0, etc.).


