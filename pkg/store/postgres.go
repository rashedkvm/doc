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
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store using a PostgreSQL connection pool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgreSQL-backed store. The dsn should be a
// full connection string like "postgresql://user:pass@host:port/db".
func NewPostgresStore(dsn string) (*PostgresStore, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres parse config: %w", err)
	}
	pool, err := pgxpool.New(context.Background(), cfg.ConnString())
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) GetCRDsForRepo(ctx context.Context, repo, tag string) ([]CRDRow, string, error) {
	var rows pgx.Rows
	var err error
	if tag == "" {
		rows, err = s.pool.Query(ctx,
			`SELECT t.name, c."group", c.version, c.kind
			 FROM tags t INNER JOIN crds c ON c.tag_id = t.id
			 WHERE LOWER(t.repo)=LOWER($1)
			   AND t.id = (SELECT id FROM tags WHERE LOWER(repo) = LOWER($1) ORDER BY time DESC LIMIT 1)`,
			repo)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT t.name, c."group", c.version, c.kind
			 FROM tags t INNER JOIN crds c ON c.tag_id = t.id
			 WHERE LOWER(t.repo)=LOWER($1) AND t.name=$2`,
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

func (s *PostgresStore) GetCRD(ctx context.Context, repo, tag, group, version, kind string) ([]byte, string, error) {
	var data []byte
	var resolvedTag string
	var err error
	if tag == "" {
		err = s.pool.QueryRow(ctx,
			`SELECT t.name, c.data::jsonb
			 FROM tags t INNER JOIN crds c ON c.tag_id = t.id
			 WHERE LOWER(t.repo)=LOWER($1)
			   AND t.id = (SELECT id FROM tags WHERE repo = $1 ORDER BY time DESC LIMIT 1)
			   AND c."group"=$2 AND c.version=$3 AND c.kind=$4`,
			repo, group, version, kind).Scan(&resolvedTag, &data)
	} else {
		err = s.pool.QueryRow(ctx,
			`SELECT t.name, c.data::jsonb
			 FROM tags t INNER JOIN crds c ON c.tag_id = t.id
			 WHERE LOWER(t.repo)=LOWER($1) AND t.name=$2
			   AND c."group"=$3 AND c.version=$4 AND c.kind=$5`,
			repo, tag, group, version, kind).Scan(&resolvedTag, &data)
	}
	if err != nil {
		return nil, "", err
	}
	return data, resolvedTag, nil
}

func (s *PostgresStore) GetRawCRDs(ctx context.Context, repo, tag string) ([][]byte, error) {
	var rows pgx.Rows
	var err error
	if tag == "" {
		rows, err = s.pool.Query(ctx,
			`SELECT c.data::jsonb
			 FROM tags t INNER JOIN crds c ON c.tag_id = t.id
			 WHERE LOWER(t.repo)=LOWER($1)
			   AND t.id = (SELECT id FROM tags WHERE LOWER(repo) = LOWER($1) ORDER BY time DESC LIMIT 1)`,
			repo)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT c.data::jsonb
			 FROM tags t INNER JOIN crds c ON c.tag_id = t.id
			 WHERE LOWER(t.repo)=LOWER($1) AND t.name=$2`,
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

func (s *PostgresStore) GetTags(ctx context.Context, repo string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT name FROM tags WHERE LOWER(repo)=LOWER($1) ORDER BY time DESC`, repo)
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

func (s *PostgresStore) GetRepos(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx,
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

func (s *PostgresStore) UpsertTag(ctx context.Context, name, repo string, timestamp time.Time) (int, error) {
	var tagID int
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM tags WHERE name=$1 AND repo=$2`, name, repo).Scan(&tagID)
	if err == nil {
		return tagID, nil
	}
	if err != pgx.ErrNoRows {
		return 0, err
	}

	err = s.pool.QueryRow(ctx,
		`INSERT INTO tags(name, repo, time) VALUES ($1, $2, $3) RETURNING id`,
		name, repo, timestamp).Scan(&tagID)
	if err != nil {
		return 0, err
	}
	return tagID, nil
}

func (s *PostgresStore) InsertCRDs(ctx context.Context, crds []CRDInsert) error {
	if len(crds) == 0 {
		return nil
	}

	query := `INSERT INTO crds("group", version, kind, tag_id, filename, data) VALUES `
	args := make([]interface{}, 0, len(crds)*6)
	for i, c := range crds {
		if i > 0 {
			query += ","
		}
		base := i * 6
		query += fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d)", base+1, base+2, base+3, base+4, base+5, base+6)
		args = append(args, c.Group, c.Version, c.Kind, c.TagID, c.Filename, c.Data)
	}
	query += " ON CONFLICT DO NOTHING"

	_, err := s.pool.Exec(ctx, query, args...)
	return err
}

func (s *PostgresStore) Close() error {
	s.pool.Close()
	return nil
}
