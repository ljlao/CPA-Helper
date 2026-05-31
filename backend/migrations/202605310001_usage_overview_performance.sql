-- +goose Up
CREATE INDEX IF NOT EXISTS ix_usage_records_failed_timestamp ON usage_records(failed, timestamp);
CREATE INDEX IF NOT EXISTS ix_usage_records_username_timestamp ON usage_records(usage_username, timestamp);
CREATE INDEX IF NOT EXISTS ix_usage_records_description_timestamp ON usage_records(api_key_description, timestamp);
CREATE INDEX IF NOT EXISTS ix_usage_records_provider_timestamp ON usage_records(provider, timestamp);
CREATE INDEX IF NOT EXISTS ix_usage_records_model_timestamp ON usage_records(model, timestamp);
CREATE INDEX IF NOT EXISTS ix_usage_records_endpoint_timestamp ON usage_records(endpoint, timestamp);
CREATE INDEX IF NOT EXISTS ix_usage_records_timestamp_source_auth ON usage_records(timestamp, source, auth);

-- +goose Down
DROP INDEX IF EXISTS ix_usage_records_timestamp_source_auth;
DROP INDEX IF EXISTS ix_usage_records_endpoint_timestamp;
DROP INDEX IF EXISTS ix_usage_records_model_timestamp;
DROP INDEX IF EXISTS ix_usage_records_provider_timestamp;
DROP INDEX IF EXISTS ix_usage_records_description_timestamp;
DROP INDEX IF EXISTS ix_usage_records_username_timestamp;
DROP INDEX IF EXISTS ix_usage_records_failed_timestamp;
