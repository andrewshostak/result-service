BEGIN;

ALTER TYPE external_match_status RENAME TO provider_match_status;

COMMIT;
