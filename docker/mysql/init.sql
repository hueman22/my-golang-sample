CREATE DATABASE IF NOT EXISTS appdb;
CREATE USER IF NOT EXISTS 'user'@'%' IDENTIFIED BY 'pass';
GRANT ALL PRIVILEGES ON appdb.* TO 'user'@'%';
FLUSH PRIVILEGES;

USE appdb;

-- ⚠️ CHỈ DÙNG CHO MÔI TRƯỜNG DEV
-- Nếu đã có bảng orders / order_items cũ, lệnh này sẽ xoá và tạo lại từ đầu.
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;

-- ==========================
-- BẢNG orders
-- ==========================
CREATE TABLE orders (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  status VARCHAR(20) NOT NULL,           -- PENDING, PAID, SHIPPED, CANCELED (theo order.Status)
  payment_method VARCHAR(50) NOT NULL,   -- "COD", "TAMARA", ...
  total_amount DECIMAL(12,2) NOT NULL DEFAULT 0.00,
  created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,

  PRIMARY KEY (id),
  KEY idx_orders_user_id (user_id)
  -- Nếu muốn thêm FK thì thêm sau khi chắc chắn bảng users tồn tại:
  -- , CONSTRAINT fk_orders_user_id FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- ==========================
-- BẢNG order_items
-- ==========================
CREATE TABLE order_items (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  order_id BIGINT UNSIGNED NOT NULL,
  product_id BIGINT UNSIGNED NOT NULL,
  product_name VARCHAR(255) NOT NULL,    -- map với OrderItem.Name
  unit_price DECIMAL(10,2) NOT NULL,     -- map với OrderItem.Price
  quantity INT NOT NULL,                 -- map với OrderItem.Quantity
  created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,

  PRIMARY KEY (id),
  KEY idx_order_items_order_id (order_id),
  KEY idx_order_items_product_id (product_id)

  -- Nếu muốn thêm FK:
  -- , CONSTRAINT fk_order_items_order_id FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
  -- , CONSTRAINT fk_order_items_product_id FOREIGN KEY (product_id) REFERENCES products(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
