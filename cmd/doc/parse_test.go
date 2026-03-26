package main

import "testing"

func TestParseRepoURL(t *testing.T) {
	tests := []struct {
		path                                       string
		host, org, repo, group, version, kind, tag string
		wantErr                                    bool
	}{
		{
			path:    "/github.com/crossplane/crossplane/cache.aws/v1alpha1/CacheCluster",
			host:    "github.com",
			org:     "crossplane",
			repo:    "crossplane",
			group:   "cache.aws",
			version: "v1alpha1",
			kind:    "CacheCluster",
		},
		{
			path:    "/github.com/crossplane/crossplane/cache.aws/v1alpha1/CacheCluster@v0.10.0",
			host:    "github.com",
			org:     "crossplane",
			repo:    "crossplane",
			group:   "cache.aws",
			version: "v1alpha1",
			kind:    "CacheCluster",
			tag:     "v0.10.0",
		},
		{
			path:    "/ghe.corp.com/platform/infra/apps/v1/MyResource@v2.0",
			host:    "ghe.corp.com",
			org:     "platform",
			repo:    "infra",
			group:   "apps",
			version: "v1",
			kind:    "MyResource",
			tag:     "v2.0",
		},
		{
			path:    "/too/short",
			wantErr: true,
		},
		{
			path:    "/",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			host, org, repo, group, version, kind, tag, err := parseRepoURL(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for path %q", tt.path)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if host != tt.host {
				t.Errorf("host = %q, want %q", host, tt.host)
			}
			if org != tt.org {
				t.Errorf("org = %q, want %q", org, tt.org)
			}
			if repo != tt.repo {
				t.Errorf("repo = %q, want %q", repo, tt.repo)
			}
			if group != tt.group {
				t.Errorf("group = %q, want %q", group, tt.group)
			}
			if version != tt.version {
				t.Errorf("version = %q, want %q", version, tt.version)
			}
			if kind != tt.kind {
				t.Errorf("kind = %q, want %q", kind, tt.kind)
			}
			if tag != tt.tag {
				t.Errorf("tag = %q, want %q", tag, tt.tag)
			}
		})
	}
}
