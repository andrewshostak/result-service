BEGIN;

DROP TRIGGER IF EXISTS teams_update_timestamp ON teams;
ALTER TABLE teams
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS updated_at;

DROP TRIGGER IF EXISTS aliases_update_timestamp ON aliases;
ALTER TABLE aliases
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS updated_at;

DROP TRIGGER IF EXISTS matches_update_timestamp ON matches;
ALTER TABLE matches
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS updated_at;

DROP TRIGGER IF EXISTS subscriptions_update_timestamp ON subscriptions;
ALTER TABLE subscriptions
    DROP COLUMN IF EXISTS updated_at;

DROP TRIGGER IF EXISTS check_result_tasks_update_timestamp ON check_result_tasks;
ALTER TABLE check_result_tasks
    DROP COLUMN IF EXISTS updated_at;

COMMIT;
