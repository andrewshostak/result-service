BEGIN;

ALTER TABLE check_result_tasks ALTER COLUMN match_id SET NOT NULL;

COMMIT;
