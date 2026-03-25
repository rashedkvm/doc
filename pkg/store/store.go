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
	"time"
)

// Store abstracts database operations for both the doc web server and the
// gitter indexer. Implementations exist for SQLite and PostgreSQL.
type Store interface {
	// GetCRDsForRepo returns all CRDs for a repo at a given tag.
	// When tag is empty, the most recent tag is used.
	// Returns the CRD rows, the resolved tag name, and any error.
	GetCRDsForRepo(ctx context.Context, repo, tag string) ([]CRDRow, string, error)

	// GetCRD returns a single CRD by its group/version/kind for a repo at a tag.
	// When tag is empty, the most recent tag is used.
	// Returns the full CRD data (as JSON bytes), the resolved tag name, and any error.
	GetCRD(ctx context.Context, repo, tag, group, version, kind string) ([]byte, string, error)

	// GetRawCRDs returns the raw JSON data for all CRDs at a repo/tag.
	// When tag is empty, the most recent tag is used.
	GetRawCRDs(ctx context.Context, repo, tag string) ([][]byte, error)

	// GetTags returns all tag names for a repo, ordered by time descending.
	GetTags(ctx context.Context, repo string) ([]string, error)

	// GetRepos returns all distinct repo paths that have been indexed.
	GetRepos(ctx context.Context) ([]string, error)

	// UpsertTag inserts a tag if it doesn't exist, or returns the existing ID.
	// Returns the tag ID.
	UpsertTag(ctx context.Context, name, repo string, timestamp time.Time) (int, error)

	// InsertCRDs batch-inserts CRD records, ignoring conflicts.
	InsertCRDs(ctx context.Context, crds []CRDInsert) error

	// Close releases any resources held by the store.
	Close() error
}

// CRDRow represents a CRD record returned from read queries.
type CRDRow struct {
	Tag      string
	Group    string
	Version  string
	Kind     string
	Filename string
	Data     []byte
}

// CRDInsert represents a CRD record to be inserted.
type CRDInsert struct {
	Group    string
	Version  string
	Kind     string
	TagID    int
	Filename string
	Data     []byte
}
