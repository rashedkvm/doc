/*
Copyright 2020 The CRDS Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const sqliteSchema = `
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
`

// SQLiteStore implements Store using an embedded SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database at dsn and applies the
// schema. The dsn is a file path; use ":memory:" for an in-memory database.
func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	if dsn == "" {
		dsn = "./doc.db"
	}
	db, err := sql.Open("sqlite", dsn+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("sqlite open %s: %w", dsn, err)
	}
	if _, err := db.Exec(sqliteSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite schema init: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) GetCRDsForRepo(ctx context.Context, repo, tag string) ([]CRDRow, string, error) {
	var rows *sql.Rows
	var err error
	if tag == "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT t.name, c."group", c.version, c.kind
			 FROM tags t INNER JOIN crds c ON c.tag_id = t.id
			 WHERE LOWER(t.repo) = LOWER(?)
			   AND t.id = (SELECT id FROM tags WHERE LOWER(repo) = LOWER(?) ORDER BY time DESC LIMIT 1)`,
			repo, repo)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT t.name, c."group", c.version, c.kind
			 FROM tags t INNER JOIN crds c ON c.tag_id = t.id
			 WHERE LOWER(t.repo) = LOWER(?) AND t.name = ?`,
			repo, tag)
	}
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var result []CRDRow
	var resolvedTag string
	for rows.Next() {
		var r CRDRow
		if err := rows.Scan(&r.Tag, &r.Group, &r.Version, &r.Kind); err != nil {
			return nil, "", err
		}
		resolvedTag = r.Tag
		result = append(result, r)
	}
	return result, resolvedTag, rows.Err()
}

func (s *SQLiteStore) GetCRD(ctx context.Context, repo, tag, group, version, kind string) ([]byte, string, error) {
	var data []byte
	var resolvedTag string
	var err error
	if tag == "" {
		err = s.db.QueryRowContext(ctx,
			`SELECT t.name, c.data
			 FROM tags t INNER JOIN crds c ON c.tag_id = t.id
			 WHERE LOWER(t.repo) = LOWER(?)
			   AND t.id = (SELECT id FROM tags WHERE LOWER(repo) = LOWER(?) ORDER BY time DESC LIMIT 1)
			   AND c."group" = ? AND c.version = ? AND c.kind = ?`,
			repo, repo, group, version, kind).Scan(&resolvedTag, &data)
	} else {
		err = s.db.QueryRowContext(ctx,
			`SELECT t.name, c.data
			 FROM tags t INNER JOIN crds c ON c.tag_id = t.id
			 WHERE LOWER(t.repo) = LOWER(?) AND t.name = ?
			   AND c."group" = ? AND c.version = ? AND c.kind = ?`,
			repo, tag, group, version, kind).Scan(&resolvedTag, &data)
	}
	if err != nil {
		return nil, "", err
	}
	return data, resolvedTag, nil
}

func (s *SQLiteStore) GetRawCRDs(ctx context.Context, repo, tag string) ([][]byte, error) {
	var rows *sql.Rows
	var err error
	if tag == "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT c.data
			 FROM tags t INNER JOIN crds c ON c.tag_id = t.id
			 WHERE LOWER(t.repo) = LOWER(?)
			   AND t.id = (SELECT id FROM tags WHERE LOWER(repo) = LOWER(?) ORDER BY time DESC LIMIT 1)`,
			repo, repo)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT c.data
			 FROM tags t INNER JOIN crds c ON c.tag_id = t.id
			 WHERE LOWER(t.repo) = LOWER(?) AND t.name = ?`,
			repo, tag)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result [][]byte
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		result = append(result, data)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) GetTags(ctx context.Context, repo string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name FROM tags WHERE LOWER(repo) = LOWER(?) ORDER BY time DESC`, repo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tags = append(tags, name)
	}
	return tags, rows.Err()
}

func (s *SQLiteStore) GetRepos(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT repo FROM tags ORDER BY repo`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []string
	for rows.Next() {
		var repo string
		if err := rows.Scan(&repo); err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

func (s *SQLiteStore) GetRepoSummaries(ctx context.Context) ([]RepoSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.repo,
		        COUNT(DISTINCT c."group" || '/' || c.version || '/' || c.kind) AS crd_count,
		        (SELECT name FROM tags WHERE repo = t.repo ORDER BY time DESC LIMIT 1) AS latest_tag
		 FROM tags t
		 INNER JOIN crds c ON c.tag_id = t.id
		 GROUP BY t.repo
		 ORDER BY t.repo`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []RepoSummary
	for rows.Next() {
		var s RepoSummary
		if err := rows.Scan(&s.Repo, &s.CRDCount, &s.LatestTag); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) UpsertTag(ctx context.Context, name, repo string, timestamp time.Time) (int, error) {
	var tagID int
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM tags WHERE name = ? AND repo = ?`, name, repo).Scan(&tagID)
	if err == nil {
		return tagID, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO tags(name, repo, time) VALUES (?, ?, ?)`, name, repo, timestamp)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *SQLiteStore) InsertCRDs(ctx context.Context, crds []CRDInsert) error {
	if len(crds) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString(`INSERT OR IGNORE INTO crds("group", version, kind, tag_id, filename, data) VALUES `)

	args := make([]interface{}, 0, len(crds)*6)
	for i, c := range crds {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("(?,?,?,?,?,?)")
		args = append(args, c.Group, c.Version, c.Kind, c.TagID, c.Filename, c.Data)
	}

	_, err := s.db.ExecContext(ctx, b.String(), args...)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
