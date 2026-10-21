BEGIN;

-- Add nullable provider column to both tables
ALTER TABLE provider_matches ADD COLUMN provider text;
ALTER TABLE provider_teams    ADD COLUMN provider text;

-- Backfill
UPDATE provider_matches SET provider = 'fotmob';
UPDATE provider_teams    SET provider = 'fotmob';

-- Make provider NOT NULL
ALTER TABLE provider_matches ALTER COLUMN provider SET NOT NULL;
ALTER TABLE provider_teams    ALTER COLUMN provider SET NOT NULL;

-- Drop old unique constraints involving provider_id
ALTER TABLE provider_matches DROP CONSTRAINT provider_matches_provider_id_provider_match_id_key;
ALTER TABLE provider_teams    DROP CONSTRAINT provider_teams_provider_id_provider_team_id_key;
ALTER TABLE provider_teams    DROP CONSTRAINT provider_teams_team_id_provider_id_key;

-- Drop old unique constraint on match_id alone and replace with (provider, match_id)
ALTER TABLE provider_matches DROP CONSTRAINT provider_matches_match_id_key;
ALTER TABLE provider_matches ADD CONSTRAINT provider_matches_provider_match_id_key UNIQUE (provider, match_id);

-- Add new unique constraints using provider string
ALTER TABLE provider_matches ADD CONSTRAINT provider_matches_provider_provider_match_id_key UNIQUE (provider, provider_match_id);
ALTER TABLE provider_teams    ADD CONSTRAINT provider_teams_provider_provider_team_id_key   UNIQUE (provider, provider_team_id);
ALTER TABLE provider_teams    ADD CONSTRAINT provider_teams_team_id_provider_key            UNIQUE (team_id, provider);

-- Drop FK constraints on provider_id
ALTER TABLE provider_matches DROP CONSTRAINT provider_matches_provider_id_fkey;
ALTER TABLE provider_teams    DROP CONSTRAINT provider_teams_provider_id_fkey;

-- Drop provider_id columns
ALTER TABLE provider_matches DROP COLUMN provider_id;
ALTER TABLE provider_teams    DROP COLUMN provider_id;

-- Drop providers table
DROP TABLE providers;

COMMIT;
