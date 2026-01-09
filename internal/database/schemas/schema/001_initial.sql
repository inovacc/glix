-- Initial schema for goinstall
-- Tracks installed Go modules and their dependencies

CREATE TABLE IF NOT EXISTS modules (
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    versions TEXT,
    dependencies TEXT,
    hash TEXT,
    time TIMESTAMP,
    PRIMARY KEY(name, version)
);

CREATE TABLE IF NOT EXISTS dependencies (
    module_name TEXT NOT NULL,
    dep_name TEXT NOT NULL,
    dep_version TEXT,
    dep_hash TEXT,
    FOREIGN KEY(module_name) REFERENCES modules(name) ON DELETE CASCADE,
    PRIMARY KEY(module_name, dep_name)
);

-- Indexes for common queries (future-proofing for monitoring)
CREATE INDEX IF NOT EXISTS idx_modules_name ON modules(name);
CREATE INDEX IF NOT EXISTS idx_modules_time ON modules(time DESC);
CREATE INDEX IF NOT EXISTS idx_dependencies_module ON dependencies(module_name);
