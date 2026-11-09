BEGIN;

ALTER TABLE check_result_tasks DROP CONSTRAINT check_result_tasks_provider_match_id_key;
ALTER TABLE check_result_tasks DROP CONSTRAINT check_result_tasks_provider_match_id_fkey;
ALTER TABLE check_result_tasks DROP COLUMN provider_match_id;

COMMIT;
