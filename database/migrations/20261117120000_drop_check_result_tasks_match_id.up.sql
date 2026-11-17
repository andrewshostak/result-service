BEGIN;

ALTER TABLE check_result_tasks DROP CONSTRAINT check_result_tasks_match_id_fkey;
ALTER TABLE check_result_tasks DROP COLUMN match_id;

COMMIT;
