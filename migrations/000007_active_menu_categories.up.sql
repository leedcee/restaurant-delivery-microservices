ALTER TABLE menu_categories
    ADD COLUMN is_active boolean NOT NULL DEFAULT true;

CREATE INDEX menu_categories_restaurant_active_idx
    ON menu_categories (restaurant_id, is_active);
