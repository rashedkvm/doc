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
	"log"
	"os"

	"sigs.k8s.io/yaml"
)

// ProviderConfig represents one entry in the "providers" configuration list.
type ProviderConfig struct {
	Host        string   `yaml:"host" json:"host"`
	Type        string   `yaml:"type" json:"type"`
	AuthSecret  string   `yaml:"auth_secret,omitempty" json:"auth_secret,omitempty"`
	AllowedOrgs []string `yaml:"allowed_orgs,omitempty" json:"allowed_orgs,omitempty"`
}

// DefaultConfigs returns the built-in provider list used when no explicit
// configuration is supplied — keeps backward compatibility with the
// existing github.com-only behavior.
func DefaultConfigs() []ProviderConfig {
	return []ProviderConfig{
		{Host: "github.com", Type: "github"},
	}
}

// valuesFile is the subset of the YAML config we care about.
type valuesFile struct {
	Providers []ProviderConfig `yaml:"providers"`
}

// LoadConfigFile reads a YAML values file and returns the providers list.
// If path is empty or the file cannot be read, it falls back to DefaultConfigs().
func LoadConfigFile(path string) []ProviderConfig {
	if path == "" {
		log.Print("CONFIG_FILE not set, using default providers")
		return DefaultConfigs()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("unable to read config file %q, using default providers: %v", path, err)
		return DefaultConfigs()
	}
	var vf valuesFile
	if err := yaml.Unmarshal(data, &vf); err != nil {
		log.Printf("unable to parse config file %q, using default providers: %v", path, err)
		return DefaultConfigs()
	}
	if len(vf.Providers) == 0 {
		log.Printf("no providers in config file %q, using default providers", path)
		return DefaultConfigs()
	}
	log.Printf("loaded %d provider(s) from %s", len(vf.Providers), path)
	return vf.Providers
}

// RegistryFromConfigs builds a Registry from a list of ProviderConfig entries.
// Unrecognised types return an error.
func RegistryFromConfigs(cfgs []ProviderConfig, secretResolver func(string) string) (*Registry, error) {
	reg := NewRegistry()
	for _, c := range cfgs {
		p, err := buildProvider(c, secretResolver)
		if err != nil {
			return nil, err
		}
		reg.Register(p)
	}
	return reg, nil
}

func buildProvider(c ProviderConfig, secretResolver func(string) string) (RepoProvider, error) {
	switch c.Type {
	case "github":
		return GitHubPublic{}, nil
	case "github-enterprise":
		token := ""
		if c.AuthSecret != "" && secretResolver != nil {
			token = secretResolver(c.AuthSecret)
			if token == "" {
				return nil, fmt.Errorf("auth secret %q is empty", c.AuthSecret)
			}
		}
		return GitHubEnterprise{Host: c.Host, Token: token}, nil
	default:
		return nil, fmt.Errorf("unknown provider type %q for host %q", c.Type, c.Host)
	}
}
