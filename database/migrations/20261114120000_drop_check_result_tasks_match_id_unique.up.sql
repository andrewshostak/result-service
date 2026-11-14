BEGIN;

UPDATE check_result_tasks crt
SET provider_match_id = pm.id
FROM provider_matches pm
WHERE pm.match_id = crt.match_id
  AND crt.provider_match_id IS NULL;

ALTER TABLE check_result_tasks DROP CONSTRAINT check_result_tasks_match_id_key;

COMMIT;
