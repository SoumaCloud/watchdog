-- name: GetClusterScheme :one
-- One cluster scheme by its text version key.
SELECT scheme_version, s2_level, station_grouping, description, created_at
FROM cluster_scheme
WHERE scheme_version = $1;
