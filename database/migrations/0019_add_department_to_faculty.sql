-- +migrate Up
ALTER TABLE faculty ADD COLUMN department_id UUID;

-- +migrate Down
ALTER TABLE faculty DROP COLUMN department_id;
