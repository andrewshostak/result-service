BEGIN;

ALTER TYPE provider_match_status RENAME TO external_match_status;

COMMIT;
