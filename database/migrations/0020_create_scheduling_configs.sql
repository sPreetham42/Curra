-- +migrate Up
CREATE TABLE scheduling_configs (
	id UUID PRIMARY KEY,
	institution_id UUID NOT NULL,
	name TEXT NOT NULL,
	config JSONB NOT NULL,
	version INT NOT NULL DEFAULT 1,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
	CONSTRAINT fk_scheduling_configs_institution FOREIGN KEY (institution_id) REFERENCES institutions(id) ON DELETE CASCADE
);

CREATE INDEX idx_scheduling_configs_institution ON scheduling_configs(institution_id);

-- +migrate Down
DROP TABLE scheduling_configs;
