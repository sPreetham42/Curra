-- +migrate Up
ALTER TABLE course_offerings ADD COLUMN academic_year_id UUID;

-- +migrate Down
ALTER TABLE course_offerings DROP COLUMN academic_year_id;
