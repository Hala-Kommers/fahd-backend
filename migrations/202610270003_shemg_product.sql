-- +goose Up
-- Imported from https://www.souqalharamain.shop/products/shemg on 2026-10-27.
-- Source checkout totals: 2 = SAR 178, 3 = SAR 209, 5 = SAR 277; free shipping.
-- Initial stock of 1,000 pieces was supplied by the store owner.
-- +goose StatementBegin
DO $$
DECLARE
    product_id_new BIGINT;
BEGIN
    -- Do not reset stock, edits, or an existing product on a repeated import.
    IF EXISTS (SELECT 1 FROM products WHERE sku = 'Hala0002074' OR slug = 'shemg') THEN
        RETURN;
    END IF;

    INSERT INTO categories (name, slug, icon, sort_order)
    VALUES ('الأشمغة', 'shemagh', 'ShoppingBag', 4)
    ON CONFLICT (slug) DO NOTHING;

    INSERT INTO products (
        title, slug, sku, status, description_short, description_long, category_id,
        price, currency, stock_total, low_stock_threshold, free_shipping, bundle_only,
        specs, faq
    ) VALUES (
        'شماغ ملكي فاخر - بالضمان الذهبي', 'shemg', 'Hala0002074', 'active',
        'شماغ بلون دم غزال ونقشة سعودية، بنسيج قطني ناعم. اختر باقتك مع شحن مجاني والدفع عند الاستلام.',
        'شماغ أحمر وأبيض بلون دم غزال، بنقشة سعودية ونسيج قطني قوي وناعم على البشرة. السعر الأساسي المعلن للقطعة ١٣٠ ر.س، وتتوفر باقات التوفير: شماغان بإجمالي ١٧٨ ر.س، ٣ شماغات بإجمالي ٢٠٩ ر.س، و٥ شماغات بإجمالي ٢٧٧ ر.س. جميع الباقات بشحن مجاني والدفع عند الاستلام.',
        (SELECT id FROM categories WHERE slug = 'shemagh'),
        130, 'SAR', 1000, 20, TRUE, TRUE,
        '[{"key":"اللون","value":"دم غزال - أحمر وأبيض"},{"key":"الخامة","value":"نسيج قطني ناعم"},{"key":"النقشة","value":"طابع سعودي"}]'::jsonb,
        '[{"question":"ما العروض المتاحة؟","answer":"شماغان بـ١٧٨ ر.س، ٣ شماغات بـ٢٠٩ ر.س، و٥ شماغات بـ٢٧٧ ر.س. هذه إجماليات الباقات، وليست أسعار القطعة."},{"question":"هل الشحن مجاني؟","answer":"نعم، الشحن مجاني لهذا المنتج إلى المدن المتاحة للتوصيل. يظهر موعد التوصيل بعد اختيار المدينة."},{"question":"كيف أدفع؟","answer":"الدفع عند الاستلام."},{"question":"ما تفاصيل الضمان الذهبي؟","answer":"المنتج معروض بالضمان الذهبي. تواصل مع المتجر لمعرفة مدة الضمان وتغطيته قبل تأكيد الطلب."},{"question":"ما سياسة الاسترجاع والاستبدال؟","answer":"الاسترجاع والاستبدال خلال ١٤ يومًا من الاستلام، بشرط أن يكون المنتج بحالته الأصلية وغير مستخدم ومع التغليف."}]'::jsonb
    ) RETURNING id INTO product_id_new;

    INSERT INTO pricing_tiers (product_id, qty, label, original_price, final_price, bundle_price)
    VALUES
        (product_id_new, 2, 'باقة الشماغين', 130, 89, 178),
        (product_id_new, 3, 'باقة ٣ شماغات', 130, 69.67, 209),
        (product_id_new, 5, 'باقة ٥ شماغات', 130, 55.40, 277);

    INSERT INTO product_images (product_id, url, is_primary, sort_order)
    VALUES
        (product_id_new, '/images/products/shemg/hero.png', TRUE, 0),
        (product_id_new, '/images/products/shemg/details.webp', FALSE, 1),
        (product_id_new, '/images/products/shemg/cotton.png', FALSE, 2),
        (product_id_new, '/images/products/shemg/offers.webp', FALSE, 3);
END $$;
-- +goose StatementEnd

-- +goose Down
-- Preserve orders and stock history while preventing sale after pricing support is rolled back.
UPDATE products SET status = 'archived', updated_at = NOW()
WHERE sku = 'Hala0002074' AND slug = 'shemg';
