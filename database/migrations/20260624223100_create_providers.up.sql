BEGIN;

CREATE TABLE IF NOT EXISTS providers (
    id         bigserial    primary key,
    name       varchar(64)  not null unique,
    created_at timestamptz  not null default now(),
    updated_at timestamptz  not null default now()
);

CREATE TRIGGER providers_update_timestamp
    BEFORE UPDATE ON providers
    FOR EACH ROW EXECUTE PROCEDURE trigger_set_updated_at();

INSERT INTO providers (name) VALUES ('fotmob');

COMMIT;
