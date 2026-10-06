-- +goose Up
CREATE TABLE marketing_settings (
 id INTEGER PRIMARY KEY CHECK (id=1),
 tiktok_pixel_id TEXT NOT NULL DEFAULT '',
 enabled BOOLEAN NOT NULL DEFAULT FALSE
);
INSERT INTO marketing_settings(id) VALUES(1);
-- +goose Down
DROP TABLE marketing_settings;
