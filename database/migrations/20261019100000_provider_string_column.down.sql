BEGIN;

-- Recreate providers table with fotmob row
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

-- Re-add provider_id columns (nullable initially for backfill)
ALTER TABLE provider_matches ADD COLUMN provider_id bigint;
ALTER TABLE provider_teams    ADD COLUMN provider_id bigint;

-- Backfill provider_id from provider column
UPDATE provider_matches pm
SET provider_id = p.id
FROM providers p
WHERE p.name = pm.provider;

UPDATE provider_teams pt
SET provider_id = p.id
FROM providers p
WHERE p.name = pt.provider;

-- Make provider_id NOT NULL
ALTER TABLE provider_matches ALTER COLUMN provider_id SET NOT NULL;
ALTER TABLE provider_teams    ALTER COLUMN provider_id SET NOT NULL;

-- Drop new string-based unique constraints
ALTER TABLE provider_matches DROP CONSTRAINT provider_matches_provider_match_id_key;
ALTER TABLE provider_matches DROP CONSTRAINT provider_matches_provider_provider_match_id_key;
ALTER TABLE provider_teams    DROP CONSTRAINT provider_teams_provider_provider_team_id_key;
ALTER TABLE provider_teams    DROP CONSTRAINT provider_teams_team_id_provider_key;

-- Restore old unique constraints
ALTER TABLE provider_matches ADD CONSTRAINT provider_matches_match_id_key                    UNIQUE (match_id);
ALTER TABLE provider_matches ADD CONSTRAINT provider_matches_provider_id_provider_match_id_key UNIQUE (provider_id, provider_match_id);
ALTER TABLE provider_teams    ADD CONSTRAINT provider_teams_provider_id_provider_team_id_key   UNIQUE (provider_id, provider_team_id);
ALTER TABLE provider_teams    ADD CONSTRAINT provider_teams_team_id_provider_id_key            UNIQUE (team_id, provider_id);

-- Restore FK constraints
ALTER TABLE provider_matches ADD CONSTRAINT provider_matches_provider_id_fkey
    FOREIGN KEY (provider_id) REFERENCES providers (id) ON UPDATE CASCADE ON DELETE RESTRICT;

ALTER TABLE provider_teams ADD CONSTRAINT provider_teams_provider_id_fkey
    FOREIGN KEY (provider_id) REFERENCES providers (id) ON UPDATE CASCADE ON DELETE RESTRICT;

-- Drop provider columns
ALTER TABLE provider_matches DROP COLUMN provider;
ALTER TABLE provider_teams    DROP COLUMN provider;

COMMIT;
