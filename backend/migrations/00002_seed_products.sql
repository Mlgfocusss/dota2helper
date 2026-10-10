-- +goose Up
INSERT INTO products (code, title, description, price_kopecks, credits, first_purchase_only, active, sort) VALUES
    ('first', 'Первый разбор', 'Первый матч со скидкой', 6700, 1, true, true, 1),
    ('pack6', '6 разборов', 'Пакет без срока действия', 49900, 6, false, true, 2)
ON CONFLICT (code) DO NOTHING;

-- +goose Down
DELETE FROM products WHERE code IN ('first', 'pack6');
