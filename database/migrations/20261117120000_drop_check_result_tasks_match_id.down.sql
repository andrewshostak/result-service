BEGIN;

ALTER TABLE check_result_tasks ADD COLUMN match_id bigint;

UPDATE check_result_tasks crt
SET match_id = pm.match_id
FROM provider_matches pm
WHERE pm.id = crt.provider_match_id;

ALTER TABLE check_result_tasks
    ADD CONSTRAINT check_result_tasks_match_id_fkey
    FOREIGN KEY (match_id) REFERENCES matches (id) ON UPDATE CASCADE ON DELETE CASCADE;

COMMIT;
