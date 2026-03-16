DROP TRIGGER IF EXISTS trg_products_search_vector ON products;
DROP FUNCTION IF EXISTS products_search_vector_update();

DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS product_contacts;
DROP TABLE IF EXISTS product_images;
DROP TABLE IF EXISTS products;
