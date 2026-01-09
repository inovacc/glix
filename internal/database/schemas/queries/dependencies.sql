-- name: UpsertDependency :exec
INSERT INTO dependencies (module_name, dep_name, dep_version, dep_hash) VALUES (?, ?, ?, ?)
ON CONFLICT(module_name, dep_name) DO UPDATE SET dep_version = excluded.dep_version, dep_hash = excluded.dep_hash;

-- name: GetDependenciesByModule :many
SELECT * FROM dependencies WHERE module_name = ?;

-- name: CountDependencies :one
SELECT COUNT(*) FROM dependencies;
