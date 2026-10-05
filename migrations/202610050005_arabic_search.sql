-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION normalize_arabic(TEXT) RETURNS TEXT LANGUAGE SQL IMMUTABLE PARALLEL SAFE AS $$
 SELECT lower(translate(regexp_replace($1, '[ً-ٟـ]', '', 'g'), 'أإآٱىة', 'اااايه'));
$$;
-- +goose StatementEnd
-- +goose Down
DROP FUNCTION normalize_arabic(TEXT);
