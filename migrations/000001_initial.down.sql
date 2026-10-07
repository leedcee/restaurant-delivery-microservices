-- Remove the initial platform schema in reverse dependency order.
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS order_status_history;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS product_inventory;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS menu_categories;
DROP TABLE IF EXISTS restaurant_integrations;
DROP TABLE IF EXISTS restaurants;
DROP TYPE IF EXISTS order_status;
