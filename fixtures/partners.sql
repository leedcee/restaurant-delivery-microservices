INSERT INTO restaurants (id, external_id, name, description, cuisine, eta, rating, art)
VALUES
    ('22222222-2222-2222-2222-222222222222', 'pasta-lab', 'Паста Лаб', 'Свежая паста, которую готовят на открытой кухне', 'Итальянская · Паста', '30–40 минут', 4.7, 'pasta'),
    ('33333333-3333-3333-3333-333333333333', 'rice-and-fish', 'Рис и рыба', 'Роллы, поке и горячие блюда в японском стиле', 'Японская · Суши', '35–45 минут', 4.9, 'sushi')
ON CONFLICT (id) DO UPDATE SET
    external_id = EXCLUDED.external_id,
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    cuisine = EXCLUDED.cuisine,
    eta = EXCLUDED.eta,
    rating = EXCLUDED.rating,
    art = EXCLUDED.art,
    is_active = true,
    updated_at = now();

INSERT INTO restaurant_integrations (restaurant_id, order_endpoint, api_key_hash)
VALUES
    ('22222222-2222-2222-2222-222222222222', 'http://demo-restaurant:8081/integration/v1/orders', encode(digest('pasta-secret', 'sha256'), 'hex')),
    ('33333333-3333-3333-3333-333333333333', 'http://demo-restaurant:8081/integration/v1/orders', encode(digest('sushi-secret', 'sha256'), 'hex'))
ON CONFLICT (restaurant_id) DO UPDATE SET
    order_endpoint = EXCLUDED.order_endpoint,
    api_key_hash = EXCLUDED.api_key_hash,
    updated_at = now();

INSERT INTO menu_categories (id, restaurant_id, external_id, name, position)
VALUES
    ('22222222-2222-4222-8222-000000000001', '22222222-2222-2222-2222-222222222222', 'pasta', 'Паста', 1),
    ('22222222-2222-4222-8222-000000000002', '22222222-2222-2222-2222-222222222222', 'starters', 'Закуски', 2),
    ('33333333-3333-4333-8333-000000000001', '33333333-3333-3333-3333-333333333333', 'rolls', 'Роллы', 1),
    ('33333333-3333-4333-8333-000000000002', '33333333-3333-3333-3333-333333333333', 'bowls', 'Боулы', 2)
ON CONFLICT (restaurant_id, external_id) DO UPDATE SET
    name = EXCLUDED.name,
    position = EXCLUDED.position,
    updated_at = now();

INSERT INTO products (id, restaurant_id, category_id, external_id, name, description, price_minor)
VALUES
    ('22222222-2222-4222-8222-000000000101', '22222222-2222-2222-2222-222222222222', '22222222-2222-4222-8222-000000000001', 'carbonara', 'Карбонара', 'Спагетти, бекон, пармезан и сливочный соус', 52000),
    ('22222222-2222-4222-8222-000000000102', '22222222-2222-2222-2222-222222222222', '22222222-2222-4222-8222-000000000001', 'pesto', 'Паста песто', 'Тальятелле, базилик, кедровый орех и пармезан', 47000),
    ('22222222-2222-4222-8222-000000000103', '22222222-2222-2222-2222-222222222222', '22222222-2222-4222-8222-000000000002', 'bruschetta', 'Брускетта', 'Томаты, базилик и хрустящий хлеб', 29000),
    ('33333333-3333-4333-8333-000000000101', '33333333-3333-3333-3333-333333333333', '33333333-3333-4333-8333-000000000001', 'salmon-roll', 'Ролл с лососем', 'Лосось, рис, нори и сливочный сыр', 56000),
    ('33333333-3333-4333-8333-000000000102', '33333333-3333-3333-3333-333333333333', '33333333-3333-4333-8333-000000000001', 'tempura-roll', 'Темпура ролл', 'Креветка, авокадо и соус спайси', 61000),
    ('33333333-3333-4333-8333-000000000103', '33333333-3333-3333-3333-333333333333', '33333333-3333-4333-8333-000000000002', 'salmon-poke', 'Поке с лососем', 'Рис, лосось, эдамаме, авокадо и манго', 59000)
ON CONFLICT (restaurant_id, external_id) DO UPDATE SET
    category_id = EXCLUDED.category_id,
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    price_minor = EXCLUDED.price_minor,
    is_active = true,
    updated_at = now();

INSERT INTO product_inventory (product_id, quantity, is_available)
VALUES
    ('22222222-2222-4222-8222-000000000101', 24, true),
    ('22222222-2222-4222-8222-000000000102', 18, true),
    ('22222222-2222-4222-8222-000000000103', 30, true),
    ('33333333-3333-4333-8333-000000000101', 20, true),
    ('33333333-3333-4333-8333-000000000102', 16, true),
    ('33333333-3333-4333-8333-000000000103', 22, true)
ON CONFLICT (product_id) DO UPDATE SET
    quantity = EXCLUDED.quantity,
    is_available = EXCLUDED.is_available,
    updated_at = now();
