-- +goose NO TRANSACTION
-- +goose Up
PRAGMA foreign_keys = OFF;

CREATE TABLE IF NOT EXISTS usage_record_payloads (
	usage_record_id INTEGER PRIMARY KEY,
	raw_json TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	FOREIGN KEY(usage_record_id) REFERENCES usage_records(id) ON DELETE CASCADE
);

INSERT OR IGNORE INTO usage_record_payloads (usage_record_id, raw_json, created_at)
SELECT id, COALESCE(raw_json, '{}'), COALESCE(CAST(created_at AS TEXT), datetime('now'))
FROM usage_records;

CREATE TABLE usage_records_slim (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at DATETIME NOT NULL,
	timestamp DATETIME NOT NULL,
	usage_username VARCHAR(120),
	api_key_description VARCHAR(240),
	provider VARCHAR(120),
	model VARCHAR(180),
	reasoning_effort VARCHAR(80),
	endpoint VARCHAR(240),
	source VARCHAR(120),
	source_account VARCHAR(320),
	request_id VARCHAR(240),
	auth VARCHAR(120),
	auth_index VARCHAR(500),
	latency_ms REAL,
	ttft_ms REAL,
	failed BOOLEAN NOT NULL DEFAULT 0,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	cached_tokens INTEGER NOT NULL DEFAULT 0,
	cache_read_tokens INTEGER NOT NULL DEFAULT 0,
	cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
	reasoning_tokens INTEGER NOT NULL DEFAULT 0,
	total_tokens INTEGER NOT NULL DEFAULT 0,
	dedupe_key VARCHAR(80) NOT NULL UNIQUE
);

INSERT INTO usage_records_slim (
	id, created_at, timestamp, usage_username, api_key_description, provider, model,
	reasoning_effort, endpoint, source, source_account, request_id, auth, auth_index,
	latency_ms, ttft_ms, failed, input_tokens, output_tokens, cached_tokens,
	cache_read_tokens, cache_creation_tokens, reasoning_tokens, total_tokens, dedupe_key
)
SELECT
	id, created_at, timestamp, usage_username, api_key_description, provider, model,
	reasoning_effort, endpoint, source, source_account, request_id, auth, auth_index,
	latency_ms, ttft_ms, failed, input_tokens, output_tokens, cached_tokens,
	cache_read_tokens, cache_creation_tokens, reasoning_tokens, total_tokens, dedupe_key
FROM usage_records;

DROP TABLE usage_records;
ALTER TABLE usage_records_slim RENAME TO usage_records;

CREATE INDEX IF NOT EXISTS ix_usage_records_timestamp ON usage_records(timestamp);
CREATE INDEX IF NOT EXISTS ix_usage_records_usage_username ON usage_records(usage_username);
CREATE INDEX IF NOT EXISTS ix_usage_records_provider ON usage_records(provider);
CREATE INDEX IF NOT EXISTS ix_usage_records_model ON usage_records(model);
CREATE INDEX IF NOT EXISTS ix_usage_records_endpoint ON usage_records(endpoint);
CREATE INDEX IF NOT EXISTS ix_usage_records_failed ON usage_records(failed);
CREATE INDEX IF NOT EXISTS ix_usage_records_source_account_timestamp ON usage_records(source_account, timestamp);
CREATE INDEX IF NOT EXISTS ix_usage_records_auth_index_timestamp ON usage_records(auth_index, timestamp);
CREATE INDEX IF NOT EXISTS ix_usage_records_failed_timestamp ON usage_records(failed, timestamp);
CREATE INDEX IF NOT EXISTS ix_usage_records_username_timestamp ON usage_records(usage_username, timestamp);
CREATE INDEX IF NOT EXISTS ix_usage_records_description_timestamp ON usage_records(api_key_description, timestamp);
CREATE INDEX IF NOT EXISTS ix_usage_records_provider_timestamp ON usage_records(provider, timestamp);
CREATE INDEX IF NOT EXISTS ix_usage_records_model_timestamp ON usage_records(model, timestamp);
CREATE INDEX IF NOT EXISTS ix_usage_records_endpoint_timestamp ON usage_records(endpoint, timestamp);
CREATE INDEX IF NOT EXISTS ix_usage_records_timestamp_source_auth ON usage_records(timestamp, source, auth);

PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;

ALTER TABLE usage_records ADD COLUMN raw_json TEXT NOT NULL DEFAULT '{}';

UPDATE usage_records
SET raw_json = (
	SELECT usage_record_payloads.raw_json
	FROM usage_record_payloads
	WHERE usage_record_payloads.usage_record_id = usage_records.id
)
WHERE EXISTS (
	SELECT 1
	FROM usage_record_payloads
	WHERE usage_record_payloads.usage_record_id = usage_records.id
);

DROP TABLE IF EXISTS usage_record_payloads;

PRAGMA foreign_keys = ON;
