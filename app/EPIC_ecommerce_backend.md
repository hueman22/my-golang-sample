# EPIC: Golang REST API backend for a simple ecommerce application

## 1. Goal

Build a backend service in Go (Golang) for a simple ecommerce application, exposed via REST APIs, running in Docker with MySQL (and later supporting PostgreSQL). The architecture should follow DDD and SOLID principles and use clean design patterns.

The system should support:
- Admins managing roles, users, categories, products, and orders.
- Guests/users browsing products, managing their cart, and checking out using Tamara or Cash on Delivery (COD).
- Proper authentication and authorization so that guest users cannot access admin APIs.

---

## 2. Tech & Architecture

- Language: Go (Golang)
- Architecture: DDD (Domain, Usecase/Application, Infrastructure/Repository, Transport/HTTP)
- Databases: MySQL (primary), with a structure that can be adapted to PostgreSQL.
- Deployment: Docker (docker-compose for app + DB)
- API style: REST (GraphQL may be added later, but not in this task)
- Testing: Unit tests + Feature/Integration tests for all main endpoints.

Suggested high-level structure:

- `cmd/api` – application entrypoint (HTTP server, wiring)
- `internal/domain/*` – entities, value objects, domain services, domain errors
- `internal/usecase/*` – application services / business logic
- `internal/repository/*` – repository interfaces + MySQL implementations
- `internal/transport/http/*` – HTTP handlers, routing, DTOs (request/response)

---

## 3. Domains and User Stories

### 3.1 User Roles (user_roles)

**User Story**  
As an Admin, I need to be able to create, read, update, and delete user roles (CRUD) so that I can define different types of users in the system.

**Requirements**
- Admin can:
  - Create user_role
  - List all user_roles
  - Get user_role by id
  - Update user_role
  - Delete user_role (subject to business rules, e.g., cannot delete if in use)
- Validate input fields: `code`, `name`, `description`.
- `code` should be unique.
- These roles will be used when creating users and for access control.

### 3.2 Users

**User Story**  
As an Admin, I need to be able to create, read, update, and delete users so that I can manage who can access the system and with which role.

**Requirements**
- Admin can:
  - Create users with a given role (linked to user_roles).
  - List users.
  - Get a single user.
  - Update user (name, email, role, etc.).
  - Delete user.
- Validate:
  - Email uniqueness.
  - Role reference must exist (user_role_id or role_code must be valid).
- Passwords must be stored as secure hashes.
- Relations:
  - `users.user_role_id` → `user_roles.id`.

### 3.3 Categories

**User Story**  
As an Admin, I need to manage product categories (CRUD) so that products can be organized and browsed more easily.

**Requirements**
- Admin can:
  - Create category.
  - List categories.
  - Get category details.
  - Update category.
  - Delete category (if no blocking business rule).
- Validate fields: non-empty name, optional slug, uniqueness constraints.

### 3.4 Products

**User Story**  
As an Admin, I need to manage products so that they can be displayed and sold in the application.

**Requirements**
- Admin can:
  - Create product (linked to a category).
  - List products (admin view).
  - Get product details.
  - Update product.
  - Delete product.
- Product fields: `name`, `description`, `price`, `stock`, `category_id`, `status` (active/inactive), etc.
- Relations:
  - `products.category_id` → `categories.id`.
- Input validation: required fields, positive price/stock, valid category id.

### 3.5 Authentication (Login)

**User Story**  
As a guest/user, I need to be able to log in to the application so that I can access protected features (cart, checkout, order history, etc.).

**Requirements**
- Endpoint to log in with email + password.
- Return a JWT (or similar token) that encodes:
  - user id
  - role information (e.g., role_code or role_id).
- This token is required for:
  - Admin APIs.
  - Customer APIs (cart, checkout, orders).

### 3.6 Browsing Products (Guest/User)

**User Story**  
As a guest/user, I need to be able to browse products and view product details so that I can decide what to buy.

**Requirements**
- Public endpoints:
  - List products (with pagination/filter, if possible).
  - List products by category.
  - Get product details by id.
- No authentication required for basic browsing.

### 3.7 Cart (Customer)

**User Story**  
As a logged-in user, I need to be able to add products to my cart and view the cart so that I can prepare my order before checkout.

**Requirements**
- Only authenticated users can use cart endpoints.
- Endpoints:
  - Add product to cart (product_id, quantity).
  - Optionally update or remove items.
  - View current cart items.
- Cart is associated to the logged-in user (using user id from token).

### 3.8 Checkout (Tamara / COD)

**User Story**  
As a logged-in user, I need to be able to checkout the products in my cart using Tamara payments or Cash on Delivery so that I can complete my purchase.

**Requirements**
- Checkout endpoint:
  - Path: `/api/v1/me/checkout` (or similar).
  - Accept a `payment_method` field: `"TAMARA"` or `"COD"`.
  - Validate cart is not empty.
  - Create an `order` and related `order_items` from the cart.
  - Clear the cart after successful checkout.
- Tamara integration can be mocked or abstracted behind a payment service interface for now.

### 3.9 Orders (Admin)

**User Story**  
As an Admin, I need to be able to view orders and update their status so that I can manage order fulfillment.

**Requirements**
- Admin-only endpoints:
  - List orders (with optional filters/pagination).
  - Get order details by id.
  - Update order status (e.g. PENDING, PAID, SHIPPED, CANCELLED).
- Orders are created during checkout.

### 3.10 Access Control

**User Story**  
As a system owner, I need to ensure guest users cannot call admin APIs so that only authorized users can manage critical resources.

**Requirements**
- All `/api/v1/admin/*` endpoints:
  - Require valid authentication token.
  - Require admin-level role (e.g., ADMIN or SUPER_ADMIN).
- Guest/unauthenticated users:
  - Should receive 401/403 when calling admin endpoints.
- Customers (non-admin) should also be rejected from admin endpoints.

---

## 4. High-level API Endpoints (Suggested)

### Auth

- `POST   /api/v1/auth/login`

### Admin – User Roles

- `GET    /api/v1/admin/user-roles`
- `POST   /api/v1/admin/user-roles`
- `GET    /api/v1/admin/user-roles/:id`
- `PATCH  /api/v1/admin/user-roles/:id`
- `DELETE /api/v1/admin/user-roles/:id`

### Admin – Users

- `GET    /api/v1/admin/users`
- `POST   /api/v1/admin/users`
- `GET    /api/v1/admin/users/:id`
- `PATCH  /api/v1/admin/users/:id`
- `DELETE /api/v1/admin/users/:id`

### Admin – Categories

- `GET    /api/v1/admin/categories`
- `POST   /api/v1/admin/categories`
- `GET    /api/v1/admin/categories/:id`
- `PATCH  /api/v1/admin/categories/:id`
- `DELETE /api/v1/admin/categories/:id`

### Admin – Products

- `GET    /api/v1/admin/products`
- `POST   /api/v1/admin/products`
- `GET    /api/v1/admin/products/:id`
- `PATCH  /api/v1/admin/products/:id`
- `DELETE /api/v1/admin/products/:id`

### Guest/User – Products

- `GET    /api/v1/products`
- `GET    /api/v1/products/:id`
- `GET    /api/v1/categories/:id/products`

### Customer – Cart

- `GET    /api/v1/me/cart`
- `POST   /api/v1/me/cart/items`
- `PATCH  /api/v1/me/cart/items/:id`      (optional)
- `DELETE /api/v1/me/cart/items/:id`     (optional)

### Customer – Checkout

- `POST   /api/v1/me/checkout`           (Tamara or COD)

### Admin – Orders

- `GET    /api/v1/admin/orders`
- `GET    /api/v1/admin/orders/:id`
- `PATCH  /api/v1/admin/orders/:id`      (update status)

---

## 5. Acceptance Criteria (A/C)

1. **Docker**
   - The Go application runs inside Docker together with MySQL using docker-compose.
   - The service is reachable (e.g. on `http://localhost:20000`).

2. **User Roles**
   - CRUD endpoints for `user_roles` are implemented.
   - Unit tests and feature/integration tests exist and pass:
     - create, list, show, update, delete
     - invalid/edge cases (invalid input, duplicate codes, not found id, etc.).

3. **Users**
   - CRUD endpoints for `users` are implemented and use `user_roles`.
   - Unit and feature tests cover:
     - create, list, show, update, delete
     - validation (email unique, valid role).

4. **Categories**
   - CRUD endpoints for `categories` implemented.
   - Unit and feature tests cover all operations and error cases.

5. **Products**
   - CRUD endpoints for `products` implemented.
   - Products are correctly linked to categories.
   - Unit and feature tests cover all operations and common edge cases.

6. **Product Browsing**
   - Public endpoints allow listing products, listing by category, and viewing details.
   - Feature tests confirm guest access works as expected.

7. **Cart & Checkout**
   - Endpoints for adding products to cart, viewing cart, and checking out (Tamara/COD) are implemented end-to-end.
   - Unit and feature tests cover:
     - empty cart
     - invalid payment method
     - successful checkout.

8. **Orders (Admin)**
   - Admin endpoints to list orders, view order details, and update status are implemented.
   - Protected by authentication & authorization.
   - Unit and feature tests cover these operations.

9. **Access Control**
   - Guest/unauthenticated users and non-admin users cannot access `/api/v1/admin/*` endpoints.
   - Tests verify that forbidden access returns proper HTTP codes (401/403).
