BEGIN;

ALTER TABLE check_result_tasks ADD CONSTRAINT check_result_tasks_match_id_key UNIQUE (match_id);

COMMIT;
