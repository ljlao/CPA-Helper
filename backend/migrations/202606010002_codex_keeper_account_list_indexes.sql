-- +goose Up
CREATE INDEX IF NOT EXISTS ix_codex_keeper_auth_states_disabled_account
	ON codex_keeper_auth_states(disabled, auth_name);

CREATE INDEX IF NOT EXISTS ix_codex_keeper_auth_states_account_type
	ON codex_keeper_auth_states(account_type);

CREATE INDEX IF NOT EXISTS ix_codex_keeper_auth_states_priority
	ON codex_keeper_auth_states(priority);

CREATE INDEX IF NOT EXISTS ix_codex_keeper_auth_states_last_status_code
	ON codex_keeper_auth_states(last_status_code);

CREATE INDEX IF NOT EXISTS ix_codex_keeper_auth_states_last_checked_at
	ON codex_keeper_auth_states(last_checked_at);

-- +goose Down
DROP INDEX IF EXISTS ix_codex_keeper_auth_states_last_checked_at;
DROP INDEX IF EXISTS ix_codex_keeper_auth_states_last_status_code;
DROP INDEX IF EXISTS ix_codex_keeper_auth_states_priority;
DROP INDEX IF EXISTS ix_codex_keeper_auth_states_account_type;
DROP INDEX IF EXISTS ix_codex_keeper_auth_states_disabled_account;
