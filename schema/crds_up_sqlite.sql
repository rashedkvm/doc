-- SQLite schema for doc application.
-- This file is for documentation purposes. The actual schema is embedded in
-- pkg/store/sqlite.go and auto-applied at process startup.

CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    repo TEXT NOT NULL,
    time DATETIME NOT NULL,
    UNIQUE(name, repo)
);

CREATE TABLE IF NOT EXISTS crds (
    "group" TEXT NOT NULL,
    version TEXT NOT NULL,
    kind TEXT NOT NULL,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    data TEXT NOT NULL,
    PRIMARY KEY(tag_id, "group", version, kind)
);
