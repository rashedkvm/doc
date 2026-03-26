package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitHubPublic(t *testing.T) {
	p := GitHubPublic{}
	if p.Name() != "github.com" {
		t.Fatalf("expected github.com, got %s", p.Name())
	}
	want := "https://github.com/org/repo"
	if got := p.CloneURL("org", "repo"); got != want {
		t.Fatalf("CloneURL = %s, want %s", got, want)
	}
	if p.Auth() != nil {
		t.Fatal("expected nil auth for public provider")
	}
	browse := p.BrowseURL("org", "repo", "v1.0")
	if browse != "https://github.com/org/repo/tree/v1.0" {
		t.Fatalf("BrowseURL = %s", browse)
	}
	browseDefault := p.BrowseURL("org", "repo", "")
	if browseDefault != "https://github.com/org/repo/tree/master" {
		t.Fatalf("BrowseURL default = %s", browseDefault)
	}
}

func TestGitHubEnterprise(t *testing.T) {
	p := GitHubEnterprise{Host: "github.corp.com", Token: "tok123"}
	if p.Name() != "github.corp.com" {
		t.Fatalf("expected github.corp.com, got %s", p.Name())
	}
	want := "https://github.corp.com/team/project"
	if got := p.CloneURL("team", "project"); got != want {
		t.Fatalf("CloneURL = %s, want %s", got, want)
	}
	if p.Auth() == nil {
		t.Fatal("expected non-nil auth for GHE with token")
	}

	noToken := GitHubEnterprise{Host: "github.corp.com"}
	if noToken.Auth() != nil {
		t.Fatal("expected nil auth for GHE without token")
	}
}

func TestRegistry(t *testing.T) {
	reg := NewRegistry()
	reg.Register(GitHubPublic{})
	reg.Register(GitHubEnterprise{Host: "ghe.corp.com"})

	if _, err := reg.Resolve("github.com"); err != nil {
		t.Fatalf("resolve github.com: %v", err)
	}
	if _, err := reg.Resolve("ghe.corp.com"); err != nil {
		t.Fatalf("resolve ghe.corp.com: %v", err)
	}
	if _, err := reg.Resolve("unknown.com"); err == nil {
		t.Fatal("expected error for unknown host")
	}

	hosts := reg.Hosts()
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
}

func TestRegistryFromConfigs(t *testing.T) {
	cfgs := []ProviderConfig{
		{Host: "github.com", Type: "github"},
		{Host: "ghe.corp.com", Type: "github-enterprise", AuthSecret: "MY_TOKEN"},
	}
	resolver := func(key string) string {
		if key == "MY_TOKEN" {
			return "secret-value"
		}
		return ""
	}
	reg, err := RegistryFromConfigs(cfgs, resolver)
	if err != nil {
		t.Fatalf("RegistryFromConfigs: %v", err)
	}

	p, err := reg.Resolve("ghe.corp.com")
	if err != nil {
		t.Fatalf("resolve ghe: %v", err)
	}
	ghe, ok := p.(GitHubEnterprise)
	if !ok {
		t.Fatal("expected GitHubEnterprise type")
	}
	if ghe.Token != "secret-value" {
		t.Fatalf("token = %q, want %q", ghe.Token, "secret-value")
	}
}

func TestRegistryFromConfigs_UnknownType(t *testing.T) {
	cfgs := []ProviderConfig{
		{Host: "gitlab.com", Type: "gitlab"},
	}
	if _, err := RegistryFromConfigs(cfgs, nil); err == nil {
		t.Fatal("expected error for unknown provider type")
	}
}

func TestDefaultConfigs(t *testing.T) {
	cfgs := DefaultConfigs()
	if len(cfgs) != 1 || cfgs[0].Host != "github.com" {
		t.Fatalf("unexpected default configs: %+v", cfgs)
	}
}

func TestLoadConfigFile(t *testing.T) {
	t.Run("empty path falls back to defaults", func(t *testing.T) {
		cfgs := LoadConfigFile("")
		if len(cfgs) != 1 || cfgs[0].Host != "github.com" {
			t.Fatalf("expected default configs, got %+v", cfgs)
		}
	})

	t.Run("missing file falls back to defaults", func(t *testing.T) {
		cfgs := LoadConfigFile("/nonexistent/path.yml")
		if len(cfgs) != 1 || cfgs[0].Host != "github.com" {
			t.Fatalf("expected default configs, got %+v", cfgs)
		}
	})

	t.Run("valid file with providers", func(t *testing.T) {
		content := `
providers:
  - host: "github.com"
    type: "github"
  - host: "ghe.corp.com"
    type: "github-enterprise"
    auth_secret: "GHE_TOKEN"
`
		f := writeTemp(t, "values-*.yml", content)
		cfgs := LoadConfigFile(f)
		if len(cfgs) != 2 {
			t.Fatalf("expected 2 providers, got %d", len(cfgs))
		}
		if cfgs[1].Host != "ghe.corp.com" {
			t.Errorf("host = %q, want %q", cfgs[1].Host, "ghe.corp.com")
		}
		if cfgs[1].AuthSecret != "GHE_TOKEN" {
			t.Errorf("auth_secret = %q, want %q", cfgs[1].AuthSecret, "GHE_TOKEN")
		}
	})

	t.Run("valid file with allowed_orgs", func(t *testing.T) {
		content := `
providers:
  - host: "ghe.corp.com"
    type: "github-enterprise"
    auth_secret: "TOK"
    allowed_orgs:
      - "platform-team"
      - "infra"
`
		f := writeTemp(t, "values-orgs-*.yml", content)
		cfgs := LoadConfigFile(f)
		if len(cfgs) != 1 {
			t.Fatalf("expected 1 provider, got %d", len(cfgs))
		}
		if len(cfgs[0].AllowedOrgs) != 2 {
			t.Fatalf("expected 2 allowed_orgs, got %d", len(cfgs[0].AllowedOrgs))
		}
	})

	t.Run("empty providers list falls back to defaults", func(t *testing.T) {
		content := `
providers: []
`
		f := writeTemp(t, "values-empty-*.yml", content)
		cfgs := LoadConfigFile(f)
		if len(cfgs) != 1 || cfgs[0].Host != "github.com" {
			t.Fatalf("expected default configs for empty providers, got %+v", cfgs)
		}
	})

	t.Run("malformed YAML falls back to defaults", func(t *testing.T) {
		content := `not: [valid: yaml: {`
		f := writeTemp(t, "values-bad-*.yml", content)
		cfgs := LoadConfigFile(f)
		if len(cfgs) != 1 || cfgs[0].Host != "github.com" {
			t.Fatalf("expected default configs for bad YAML, got %+v", cfgs)
		}
	})

	t.Run("file with no providers key falls back to defaults", func(t *testing.T) {
		content := `
registry: "docker.io"
image_tag: "latest"
`
		f := writeTemp(t, "values-noprov-*.yml", content)
		cfgs := LoadConfigFile(f)
		if len(cfgs) != 1 || cfgs[0].Host != "github.com" {
			t.Fatalf("expected default configs when providers key missing, got %+v", cfgs)
		}
	})

	t.Run("file with extra keys ignores them", func(t *testing.T) {
		content := `
registry: "docker.io"
db_driver: "sqlite"
providers:
  - host: "github.com"
    type: "github"
`
		f := writeTemp(t, "values-extra-*.yml", content)
		cfgs := LoadConfigFile(f)
		if len(cfgs) != 1 || cfgs[0].Host != "github.com" {
			t.Fatalf("expected 1 provider, got %+v", cfgs)
		}
	})
}

func writeTemp(t *testing.T, pattern, content string) string {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, pattern)
	tmp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := tmp.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f = tmp.Name()
	tmp.Close()
	return f
}
