-- +goose Up
UPDATE categories SET name = 'عطور', icon = 'Sparkles' WHERE slug = 'perfumes';
UPDATE categories SET name = 'العناية بالجسم', icon = 'Leaf' WHERE slug = 'body-care';
UPDATE categories SET name = 'مجموعات الهدايا', icon = 'Gift' WHERE slug = 'gift-sets';

INSERT INTO categories (name, slug, icon, description, sort_order)
VALUES
  ('سماعات', 'headphones', 'Headphones', 'سماعات وإكسسوارات الصوت', 4),
  ('ساعات', 'watches', 'Watch', 'ساعات اليد', 5),
  ('إكسسوارات', 'accessories', 'Gem', 'إكسسوارات متنوعة', 6),
  ('حقائب', 'bags', 'Briefcase', 'حقائب للاستخدام اليومي', 7),
  ('كاميرات', 'cameras', 'Camera', 'كاميرات ومستلزمات التصوير', 8),
  ('قيمنق', 'gaming', 'Gamepad2', 'أجهزة وإكسسوارات الألعاب', 9),
  ('أجهزة ذكية', 'smart-devices', 'Lightbulb', 'أجهزة ذكية', 10),
  ('كوزماتك', 'cosmetics', 'Paintbrush', 'مستحضرات التجميل والمكياج', 11)
ON CONFLICT (slug) DO NOTHING;

-- +goose Down
DELETE FROM categories WHERE slug IN ('headphones', 'watches', 'accessories', 'bags', 'cameras', 'gaming', 'smart-devices', 'cosmetics')
  AND NOT EXISTS (SELECT 1 FROM products WHERE products.category_id = categories.id);
UPDATE categories SET name = 'Perfumes', icon = 'sparkles' WHERE slug = 'perfumes';
UPDATE categories SET name = 'Body Care', icon = 'leaf' WHERE slug = 'body-care';
UPDATE categories SET name = 'Gift Sets', icon = 'gift' WHERE slug = 'gift-sets';
