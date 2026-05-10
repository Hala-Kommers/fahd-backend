-- +goose Up

INSERT INTO users (username, name, email, password_hash, role)
VALUES ('admin', 'Admin User', 'admin@fahd.local', '$2a$12$P/GdDUYY6kNL3Qtz.WygBe7XvjbdhU3SEgDTmjysRHfRHHnirh0Rq', 'admin')
ON CONFLICT (username) DO NOTHING;

INSERT INTO categories (name, slug, icon, description, sort_order)
VALUES
  ('Perfumes', 'perfumes', 'sparkles', 'Premium perfumes collection', 1),
  ('Body Care', 'body-care', 'leaf', 'Body and skincare essentials', 2),
  ('Gift Sets', 'gift-sets', 'gift', 'Curated gift bundles', 3)
ON CONFLICT (slug) DO NOTHING;

INSERT INTO products (
  title, slug, sku, status, description_short, description_long, category_id,
  cost, price, compare_at, currency, inventory_mode, stock_total, low_stock_threshold,
  is_featured, has_variants, variant_options, specs, faq, usage_instructions,
  sales_count, rating
)
SELECT
  'Oud Signature', 'oud-signature', 'P-1001', 'active',
  'Warm oud blend with amber notes',
  'A refined oud fragrance suitable for daily and evening use.',
  c.id,
  65.00, 120.00, 160.00, 'SAR', 'global', 50, 5,
  TRUE, FALSE, '[]'::jsonb,
  '[{"key":"volume","value":"100ml"}]'::jsonb,
  '[{"question":"How long does it last?","answer":"6-8 hours"}]'::jsonb,
  'Spray on pulse points from 15cm distance.',
  0, 0
FROM categories c
WHERE c.slug = 'perfumes'
ON CONFLICT (sku) DO NOTHING;

INSERT INTO product_images (product_id, url, is_primary, sort_order)
SELECT p.id, 'https://images.unsplash.com/photo-1594035910387-fea47794261f', TRUE, 0
FROM products p
WHERE p.sku = 'P-1001'
  AND NOT EXISTS (
    SELECT 1 FROM product_images i WHERE i.product_id = p.id AND i.is_primary = TRUE
  );

INSERT INTO pricing_tiers (product_id, qty, label, original_price, final_price)
SELECT p.id, 1, 'قطعة واحدة', 160.00, 120.00
FROM products p
WHERE p.sku = 'P-1001'
ON CONFLICT (product_id, qty) DO NOTHING;

INSERT INTO coupons (code, type, value, min_order, starts_at, expires_at, is_active)
VALUES ('WELCOME10', 'percentage', 10.00, 100.00, NOW(), NOW() + INTERVAL '30 days', TRUE)
ON CONFLICT (code) DO NOTHING;

INSERT INTO policies (key, title, body, content, applies_to, is_active)
VALUES
  ('shipping', 'Shipping Policy', 'Delivery in 1-3 business days for major cities.', '["Delivery in 1-3 business days", "Tracking is available"]'::jsonb, 'all', TRUE),
  ('returns', 'Return Policy', 'Returns accepted within 7 days for unopened products.', '["Returns within 7 days", "Product must be unopened"]'::jsonb, 'all', TRUE)
ON CONFLICT (key) DO NOTHING;

INSERT INTO global_faq (question, answer, sort_order, is_active)
VALUES
  ('How do I track my order?', 'Use your order number on the tracking page.', 1, TRUE),
  ('Which payment methods are supported?', 'COD and Online payments are supported.', 2, TRUE);

INSERT INTO bot_config (provider, model, enabled, persona, system, templates, closing)
SELECT
  'google',
  'gemini-2.0-flash',
  TRUE,
  '{"botName":"فهد","tone":"friendly_saudi","style":"concise","language":"ar-SA","emojiLevel":"medium"}'::jsonb,
  '{"systemPrompt":"You are Fahd store assistant","allowedSources":["catalog","product_faq","store_policies"]}'::jsonb,
  '{"welcome":"هلا 👋 أبشر..."}'::jsonb,
  '{"paymentMethodsEnabled":["cod","paymob"]}'::jsonb
WHERE NOT EXISTS (SELECT 1 FROM bot_config);

-- +goose Down
DELETE FROM pricing_tiers WHERE label = 'قطعة واحدة';
DELETE FROM product_images WHERE url = 'https://images.unsplash.com/photo-1594035910387-fea47794261f';
DELETE FROM products WHERE sku = 'P-1001';
DELETE FROM categories WHERE slug IN ('perfumes', 'body-care', 'gift-sets');
DELETE FROM coupons WHERE code = 'WELCOME10';
DELETE FROM policies WHERE key IN ('shipping', 'returns');
DELETE FROM global_faq WHERE question IN ('How do I track my order?', 'Which payment methods are supported?');
DELETE FROM bot_config WHERE model = 'gemini-2.0-flash';
DELETE FROM users WHERE username = 'admin';
