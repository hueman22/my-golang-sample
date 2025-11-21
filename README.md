## Ecommerce Backend (Golang + MySQL/PostgreSQL)

REST API phục vụ bài toán ecommerce nhỏ, triển khai theo phong cách DDD + SOLID, handler mỏng – service chứa business logic – repository chỉ làm việc với DB. Ứng dụng chạy trong Docker (MySQL + PostgreSQL) hoặc chạy trực tiếp trên máy dev.

### 1. Kiến trúc & Thư mục chính
- `app/main.go`: entrypoint, DI wiring, health check, seed SUPER_ADMIN.
- `app/internal/domain/*`: entity + rule của từng bounded context (`user`, `userrole`, `category`, `product`, `cart`, `order`…).
- `app/internal/usecase/*`: service layer/ứng dụng, xử lý validate + business rule.
- `app/internal/infra/persistence/mysql`: repository MySQL cho từng domain.
- `app/internal/infra/security`: BCrypt password service + JWT service.
- `app/internal/interface/http`: router (chi), middleware JWT, handler REST cho admin/user/guest.

Luồng xử lý: HTTP handler (decode + validate) → usecase service (logic, DI) → repository (SQL) → MySQL. JWT middleware nạp context user để enforce role.

### 2. Environment & Docker
1. **Tạo file env**
   ```bash
   cp app/env.example app/.env
   # hoặc export thủ công: export $(grep -v '^#' app/.env | xargs)
   ```
   Các biến quan trọng:
   - `APP_PORT`: port HTTP của Go app.
   - `MYSQL_DSN`, `PG_DSN`: connection string tới container DB.
   - `JWT_SECRET`: khóa ký JWT (HS256).
   - `SUPER_ADMIN_EMAIL` + `SUPER_ADMIN_PASSWORD`: dùng để seed tài khoản SUPER_ADMIN đầu tiên.

2. **Chạy docker-compose**
   ```bash
   docker compose -f docker-compose.app.yml up -d mysql postgres mailpit
   docker compose -f docker-compose.app.yml up --build app
   ```
   - MySQL listen `13306`, Postgres `15433`, app publish `20000`.
   - File init SQL nằm ở `docker/mysql/init.sql` và `docker/postgres/init.sql`.

3. **Chạy app ngoài Docker**
   ```bash
   cd app
   export $(grep -v '^#' .env | xargs)   # nếu dùng file env
   go run ./...
   ```

### 3. Database migration & seed
- `ensureTables` trong `main.go` tự động tạo bảng `user_roles`, `users`, `categories`, `products`, `cart_items`, `orders`, `order_items`.
- Bảng `user_roles` được seed 3 role mặc định: `SUPER_ADMIN`, `ADMIN`, `CUSTOMER`.
- Hàm `seedSuperAdmin` sẽ tạo user SUPER_ADMIN (nếu chưa tồn tại) dựa trên `SUPER_ADMIN_EMAIL/PASSWORD`.
- Khi cần migrate thủ công chỉ cần chạy `go run ./app` sau khi cập nhật schema, vì hàm ensureTables luôn idempotent.

### 4. Chức năng REST (tóm tắt)
- **Guest/User**
  - `POST /api/v1/auth/login`
  - `GET /api/v1/products`, `GET /api/v1/products/{id}`
  - `POST /api/v1/me/cart/items`, `GET /api/v1/me/cart`
  - `POST /api/v1/me/checkout` (`payment_method`: `COD` hoặc `TAMARA`)
- **Admin/Super Admin (`Authorization: Bearer <JWT>`):**
  - CRUD `user-roles`, `users`, `categories`, `products`
  - `GET /api/v1/admin/orders`, `GET /api/v1/admin/orders/{id}`, `PATCH /api/v1/admin/orders/{id}/status`
  - Chính sách quan trọng:
    - ADMIN **không được** tạo/promote user có `role_code = ADMIN`.
    - Vi phạm trả HTTP 422, không thay đổi DB.
    - SUPER_ADMIN không bị giới hạn.

### 5. Quy trình kiểm thử
```bash
cd app
GOPROXY=direct GOSUMDB=off GOMODCACHE=$(pwd)/.gocache GOCACHE=$(pwd)/.gocache-build \
  go test ./...
```
- `internal/usecase/user/service_test.go`: unit test mock repo → kiểm tra rule phân quyền.
- `internal/interface/http/admin_users_handler_test.go`: feature test dùng `httptest` → ADMIN tạo user role ADMIN phải nhận 422.

### 6. Ví dụ curl
```bash
# Login (nhận JWT)
curl -X POST http://localhost:20000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"super.admin@example.com","password":"ChangeMe123!"}'

# ADMIN cố tạo user role ADMIN => 422
curl -X POST http://localhost:20000/api/v1/admin/users \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Another Admin","email":"admin2@example.com","password":"Admin123!","role_code":"ADMIN"}'

# SUPER_ADMIN tạo user ADMIN => 201
curl -X POST http://localhost:20000/api/v1/admin/users \
  -H "Authorization: Bearer $SUPER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Admin OK","email":"admin.ok@example.com","password":"Super123!","role_code":"ADMIN"}'
```

### 7. Phân quyền chi tiết
- `SUPER_ADMIN`: full quyền CRUD + quản lý roles/users/orders.
- `ADMIN`: giống SUPER_ADMIN nhưng **không** thể tạo hoặc cập nhật user lên `ADMIN`.
- `CUSTOMER`: login, xem sản phẩm, quản lý giỏ hàng, checkout (COD/TAMARA mock).

### 8. Troubleshooting
- **Không tải được module Go**: kéo bằng `GOPROXY=direct GOSUMDB=off GOINSECURE=github.com,...` như script ở phần test.
- **Seed SUPER_ADMIN không chạy**: kiểm tra env `SUPER_ADMIN_EMAIL/PASSWORD` và log khởi động.
- **422 khi tạo user**: kiểm tra role executor và `role_code` target theo rule đã mô tả.
- **Checkout lỗi**: đảm bảo sản phẩm còn stock và giỏ hàng không trống (`cart_items`).

### 9. Tiếp tục phát triển
- Thêm refresh token/rotate secret.
- Hoàn thiện Postgres storage (hiện Postgres chỉ dùng cho health-check/demo DSN).
- Viết migration tool (golang-migrate) để quản lý schema ngoài code.


