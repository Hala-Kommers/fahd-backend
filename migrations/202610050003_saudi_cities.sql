-- +goose Up
UPDATE cities SET name='الرياض', sort_order=1 WHERE name='Riyadh';
UPDATE cities SET name='جدة', sort_order=2 WHERE name='Jeddah';
UPDATE cities SET name='الدمام', sort_order=3 WHERE name='Dammam';
UPDATE cities SET name='مكة المكرمة', sort_order=4 WHERE name='Makkah';
UPDATE cities SET name='المدينة المنورة', sort_order=5 WHERE name='Madinah';
INSERT INTO cities(name, sort_order) VALUES
 ('بريدة',6),('عنيزة',7),('الخبر',8),('الأحساء',9),('الجبيل',10),('الطائف',11),
 ('أبها',12),('خميس مشيط',13),('تبوك',14),('حائل',15),('عرعر',16),('رفحاء',17),
 ('جازان',18),('نجران',19),('الباحة',20),('سكاكا',21),('القريات',22),('ينبع',23),
 ('الخرج',24),('الدوادمي',25),('بيشة',26),('صبيا',27)
ON CONFLICT(name) DO NOTHING;
-- +goose Down
UPDATE cities SET name='Riyadh' WHERE name='الرياض';
UPDATE cities SET name='Jeddah' WHERE name='جدة';
UPDATE cities SET name='Dammam' WHERE name='الدمام';
UPDATE cities SET name='Makkah' WHERE name='مكة المكرمة';
UPDATE cities SET name='Madinah' WHERE name='المدينة المنورة';
