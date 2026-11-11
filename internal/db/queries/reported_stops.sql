-- name: GetReportedStop :one
-- One reported stop by ID.
SELECT reported_stop_id, agency_id, oba_stop_id
FROM reported_stop
WHERE reported_stop_id = $1;

-- name: ListReportedStopsByIDs :many
-- The reported stops with the given IDs, for the include loader. IDs with no
-- row are left out.
SELECT reported_stop_id, agency_id, oba_stop_id
FROM reported_stop
WHERE reported_stop_id = ANY(sqlc.arg('reported_stop_ids')::bigint[])
ORDER BY reported_stop_id;
