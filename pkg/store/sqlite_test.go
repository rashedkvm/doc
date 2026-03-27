package store

import (
	"context"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore(:memory:): %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func seedRepo(t *testing.T, s *SQLiteStore, repo, tag string, crds []CRDInsert) {
	t.Helper()
	ctx := context.Background()
	tagID, err := s.UpsertTag(ctx, tag, repo, time.Now())
	if err != nil {
		t.Fatalf("UpsertTag(%s, %s): %v", tag, repo, err)
	}
	for i := range crds {
		crds[i].TagID = tagID
	}
	if err := s.InsertCRDs(ctx, crds); err != nil {
		t.Fatalf("InsertCRDs: %v", err)
	}
}

func TestGetRepoSummaries_Empty(t *testing.T) {
	s := newTestStore(t)
	summaries, err := s.GetRepoSummaries(context.Background())
	if err != nil {
		t.Fatalf("GetRepoSummaries: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("expected 0 summaries on empty DB, got %d", len(summaries))
	}
}

func TestGetRepoSummaries_SingleRepo(t *testing.T) {
	s := newTestStore(t)
	seedRepo(t, s, "github.com/org/repo", "v1.0.0", []CRDInsert{
		{Group: "apps.example.com", Version: "v1", Kind: "MyApp", Filename: "myapp.yaml", Data: []byte(`{}`)},
		{Group: "cache.example.com", Version: "v1alpha1", Kind: "Redis", Filename: "redis.yaml", Data: []byte(`{}`)},
	})

	summaries, err := s.GetRepoSummaries(context.Background())
	if err != nil {
		t.Fatalf("GetRepoSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].Repo != "github.com/org/repo" {
		t.Errorf("Repo = %q, want %q", summaries[0].Repo, "github.com/org/repo")
	}
	if summaries[0].CRDCount != 2 {
		t.Errorf("CRDCount = %d, want 2", summaries[0].CRDCount)
	}
	if summaries[0].LatestTag != "v1.0.0" {
		t.Errorf("LatestTag = %q, want %q", summaries[0].LatestTag, "v1.0.0")
	}
}

func TestGetRepoSummaries_MultipleRepos(t *testing.T) {
	s := newTestStore(t)
	seedRepo(t, s, "github.com/alpha/one", "v1.0.0", []CRDInsert{
		{Group: "g1", Version: "v1", Kind: "K1", Filename: "k1.yaml", Data: []byte(`{}`)},
	})
	seedRepo(t, s, "github.com/beta/two", "v2.0.0", []CRDInsert{
		{Group: "g2", Version: "v1", Kind: "K2", Filename: "k2.yaml", Data: []byte(`{}`)},
		{Group: "g2", Version: "v1", Kind: "K3", Filename: "k3.yaml", Data: []byte(`{}`)},
		{Group: "g3", Version: "v1beta1", Kind: "K4", Filename: "k4.yaml", Data: []byte(`{}`)},
	})

	summaries, err := s.GetRepoSummaries(context.Background())
	if err != nil {
		t.Fatalf("GetRepoSummaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}

	// Results are ordered by repo name.
	if summaries[0].Repo != "github.com/alpha/one" {
		t.Errorf("summaries[0].Repo = %q, want github.com/alpha/one", summaries[0].Repo)
	}
	if summaries[0].CRDCount != 1 {
		t.Errorf("summaries[0].CRDCount = %d, want 1", summaries[0].CRDCount)
	}
	if summaries[1].Repo != "github.com/beta/two" {
		t.Errorf("summaries[1].Repo = %q, want github.com/beta/two", summaries[1].Repo)
	}
	if summaries[1].CRDCount != 3 {
		t.Errorf("summaries[1].CRDCount = %d, want 3", summaries[1].CRDCount)
	}
}

func TestGetRepoSummaries_LatestTagIsNewest(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Insert an older tag, then a newer tag for the same repo.
	oldTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newTime := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	oldTagID, err := s.UpsertTag(ctx, "v1.0.0", "github.com/org/repo", oldTime)
	if err != nil {
		t.Fatalf("UpsertTag: %v", err)
	}
	if err := s.InsertCRDs(ctx, []CRDInsert{
		{Group: "g", Version: "v1", Kind: "K", TagID: oldTagID, Filename: "k.yaml", Data: []byte(`{}`)},
	}); err != nil {
		t.Fatalf("InsertCRDs: %v", err)
	}

	newTagID, err := s.UpsertTag(ctx, "v2.0.0", "github.com/org/repo", newTime)
	if err != nil {
		t.Fatalf("UpsertTag: %v", err)
	}
	if err := s.InsertCRDs(ctx, []CRDInsert{
		{Group: "g", Version: "v1", Kind: "K", TagID: newTagID, Filename: "k.yaml", Data: []byte(`{}`)},
		{Group: "g", Version: "v1", Kind: "K2", TagID: newTagID, Filename: "k2.yaml", Data: []byte(`{}`)},
	}); err != nil {
		t.Fatalf("InsertCRDs: %v", err)
	}

	summaries, err := s.GetRepoSummaries(ctx)
	if err != nil {
		t.Fatalf("GetRepoSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].LatestTag != "v2.0.0" {
		t.Errorf("LatestTag = %q, want v2.0.0", summaries[0].LatestTag)
	}
	// CRD count is across all tags (distinct GVKs across the entire repo).
	if summaries[0].CRDCount != 2 {
		t.Errorf("CRDCount = %d, want 2", summaries[0].CRDCount)
	}
}
