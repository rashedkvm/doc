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

package provider

import (
	"fmt"
	"sync"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// RepoProvider abstracts a Git hosting platform. Implementations supply
// host-specific clone URLs and optional authentication so the rest of
// the codebase never hard-codes a particular host.
type RepoProvider interface {
	// Name returns the canonical hostname (e.g. "github.com").
	Name() string
	// CloneURL returns the HTTPS clone URL for the given org/repo.
	CloneURL(org, repo string) string
	// Auth returns go-git transport auth, or nil for public access.
	Auth() transport.AuthMethod
	// BrowseURL returns a URL a human can visit to see the repo at a tag.
	// An empty tag means the default branch.
	BrowseURL(org, repo, tag string) string
}

// Registry maps hostnames to RepoProvider instances.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]RepoProvider
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]RepoProvider)}
}

// Register adds a provider. If a provider with the same Name() already
// exists it is silently replaced.
func (r *Registry) Register(p RepoProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

// Resolve returns the provider for host, or an error if unknown.
func (r *Registry) Resolve(host string) (RepoProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[host]
	if !ok {
		return nil, fmt.Errorf("unknown provider host: %q", host)
	}
	return p, nil
}

// Hosts returns all registered hostnames.
func (r *Registry) Hosts() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.providers))
	for h := range r.providers {
		out = append(out, h)
	}
	return out
}

// --- Built-in providers ---

// GitHubPublic is the zero-config provider for public github.com repos.
type GitHubPublic struct{}

func (g GitHubPublic) Name() string { return "github.com" }
func (g GitHubPublic) CloneURL(org, repo string) string {
	return fmt.Sprintf("https://github.com/%s/%s", org, repo)
}
func (g GitHubPublic) Auth() transport.AuthMethod { return nil }

func (g GitHubPublic) BrowseURL(org, repo, tag string) string {
	if tag == "" {
		tag = "master"
	}
	return fmt.Sprintf("https://github.com/%s/%s/tree/%s", org, repo, tag)
}

// GitHubEnterprise supports a configurable GHE hostname with optional
// token-based authentication for private repositories.
type GitHubEnterprise struct {
	Host  string
	Token string
}

func (g GitHubEnterprise) Name() string { return g.Host }
func (g GitHubEnterprise) CloneURL(org, repo string) string {
	return fmt.Sprintf("https://%s/%s/%s", g.Host, org, repo)
}

func (g GitHubEnterprise) Auth() transport.AuthMethod {
	if g.Token == "" {
		return nil
	}
	return &githttp.BasicAuth{
		Username: "x-access-token",
		Password: g.Token,
	}
}

func (g GitHubEnterprise) BrowseURL(org, repo, tag string) string {
	if tag == "" {
		tag = "master"
	}
	return fmt.Sprintf("https://%s/%s/%s/tree/%s", g.Host, org, repo, tag)
}
