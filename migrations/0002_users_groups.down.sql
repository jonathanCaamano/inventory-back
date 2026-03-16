DROP INDEX IF EXISTS idx_products_group_id;

ALTER TABLE products
  DROP COLUMN IF EXISTS group_id;

DROP TABLE IF EXISTS group_memberships;
DROP TABLE IF EXISTS groups;
DROP TABLE IF EXISTS users;