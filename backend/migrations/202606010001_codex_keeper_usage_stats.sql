-- +goose Up
CREATE TABLE IF NOT EXISTS codex_keeper_account_usage_stats (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	auth_name VARCHAR(500) NOT NULL,
	email VARCHAR(320),
	account_type VARCHAR(80),
	period_type VARCHAR(10) NOT NULL CHECK (period_type IN ('day', 'week')),
	period_start DATETIME NOT NULL,
	period_end DATETIME NOT NULL,
	records INTEGER NOT NULL DEFAULT 0,
	success_records INTEGER NOT NULL DEFAULT 0,
	failed_records INTEGER NOT NULL DEFAULT 0,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	cached_tokens INTEGER NOT NULL DEFAULT 0,
	cache_read_tokens INTEGER NOT NULL DEFAULT 0,
	cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
	reasoning_tokens INTEGER NOT NULL DEFAULT 0,
	total_tokens INTEGER NOT NULL DEFAULT 0,
	estimated_cost_usd REAL NOT NULL DEFAULT 0,
	unpriced_records INTEGER NOT NULL DEFAULT 0,
	first_request_at DATETIME,
	last_request_at DATETIME,
	generated_at DATETIME NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	CONSTRAINT uq_codex_keeper_account_usage_stats_period UNIQUE (auth_name, period_type, period_start)
);

CREATE INDEX IF NOT EXISTS ix_codex_keeper_account_usage_stats_auth_period
	ON codex_keeper_account_usage_stats(auth_name, period_type, period_start);

CREATE INDEX IF NOT EXISTS ix_codex_keeper_account_usage_stats_generated
	ON codex_keeper_account_usage_stats(generated_at);

-- +goose Down
DROP INDEX IF EXISTS ix_codex_keeper_account_usage_stats_generated;
DROP INDEX IF EXISTS ix_codex_keeper_account_usage_stats_auth_period;
DROP TABLE IF EXISTS codex_keeper_account_usage_stats;
