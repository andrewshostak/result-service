BEGIN;

ALTER TABLE teams
    ADD COLUMN created_at timestamptz not null default now(),
    ADD COLUMN updated_at timestamptz not null default now();

CREATE TRIGGER teams_update_timestamp
    BEFORE UPDATE ON teams
    FOR EACH ROW EXECUTE PROCEDURE trigger_set_updated_at();

ALTER TABLE aliases
    ADD COLUMN created_at timestamptz not null default now(),
    ADD COLUMN updated_at timestamptz not null default now();

CREATE TRIGGER aliases_update_timestamp
    BEFORE UPDATE ON aliases
    FOR EACH ROW EXECUTE PROCEDURE trigger_set_updated_at();

ALTER TABLE matches
    ADD COLUMN created_at timestamptz not null default now(),
    ADD COLUMN updated_at timestamptz not null default now();

CREATE TRIGGER matches_update_timestamp
    BEFORE UPDATE ON matches
    FOR EACH ROW EXECUTE PROCEDURE trigger_set_updated_at();

ALTER TABLE subscriptions
    ADD COLUMN updated_at timestamptz not null default now();

CREATE TRIGGER subscriptions_update_timestamp
    BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE PROCEDURE trigger_set_updated_at();

ALTER TABLE check_result_tasks
    ADD COLUMN updated_at timestamptz not null default now();

CREATE TRIGGER check_result_tasks_update_timestamp
    BEFORE UPDATE ON check_result_tasks
    FOR EACH ROW EXECUTE PROCEDURE trigger_set_updated_at();

COMMIT;
