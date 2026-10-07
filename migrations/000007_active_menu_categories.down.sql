DROP INDEX IF EXISTS menu_categories_restaurant_active_idx;
ALTER TABLE menu_categories DROP COLUMN IF EXISTS is_active;
