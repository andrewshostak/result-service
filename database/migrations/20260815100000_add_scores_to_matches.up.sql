BEGIN;

ALTER TABLE matches
    ADD COLUMN home_score smallint,
    ADD COLUMN away_score smallint;

UPDATE matches
SET home_score = em.home_score,
    away_score = em.away_score
FROM external_matches em
WHERE em.match_id = matches.id
  AND matches.result_status = 'received';

COMMIT;
