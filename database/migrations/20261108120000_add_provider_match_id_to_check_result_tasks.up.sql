BEGIN;

ALTER TABLE check_result_tasks ADD COLUMN provider_match_id bigint;

ALTER TABLE check_result_tasks
    ADD CONSTRAINT check_result_tasks_provider_match_id_fkey
    FOREIGN KEY (provider_match_id) REFERENCES provider_matches (id) ON UPDATE CASCADE ON DELETE CASCADE;

ALTER TABLE check_result_tasks
    ADD CONSTRAINT check_result_tasks_provider_match_id_key UNIQUE (provider_match_id);

COMMIT;
