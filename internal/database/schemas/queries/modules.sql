-- name: UpsertModule :exec
INSERT INTO modules (name, version, versions, dependencies, hash, time) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(name, version) DO UPDATE SET hash = excluded.hash, time = excluded.time, versions = excluded.versions, dependencies = excluded.dependencies;

-- name: GetModule :one
SELECT * FROM modules WHERE name = ? AND version = ? LIMIT 1;

-- name: GetModuleByName :many
SELECT * FROM modules WHERE name = ? ORDER BY time DESC;

-- name: ListModules :many
SELECT * FROM modules ORDER BY time DESC;

-- name: CountModules :one
SELECT COUNT(*) FROM modules;

-- name: DeleteModule :exec
DELETE FROM modules WHERE name = ? AND version = ?;
