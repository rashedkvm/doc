package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crdsdev/doc/pkg/models"
	"github.com/unrolled/render"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

func projectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	return filepath.Join(wd, "..", "..")
}

func newTestRenderer(t *testing.T) *testRenderer {
	t.Helper()
	root := projectRoot(t)
	r := render.New(render.Options{
		Extensions: []string{".html"},
		Directory:  filepath.Join(root, "template"),
		Layout:     "layout",
		Funcs: []template.FuncMap{
			{
				"plusParent": func(p string, s map[string]apiextensions.JSONSchemaProps) *SchemaPlusParent {
					return &SchemaPlusParent{Parent: p, Schema: s}
				},
				"toJSON": func(v interface{}) template.JS {
					b, err := json.Marshal(v)
					if err != nil {
						return template.JS("{}")
					}
					return template.JS(b)
				},
			},
		},
	})
	return &testRenderer{r: r}
}

type testRenderer struct {
	r *render.Render
}

func (tr *testRenderer) render(t *testing.T, tmpl string, data interface{}) string {
	t.Helper()
	w := httptest.NewRecorder()
	if err := tr.r.HTML(w, http.StatusOK, tmpl, data); err != nil {
		t.Fatalf("template %q render failed: %v", tmpl, err)
	}
	return w.Body.String()
}

// --- Template rendering tests ---

func TestHomeTemplate(t *testing.T) {
	tr := newTestRenderer(t)

	t.Run("renders without error", func(t *testing.T) {
		data := homeData{
			Page: pageData{Title: "Doc", DisableNavBar: true},
		}
		body := tr.render(t, "home", data)
		if !strings.Contains(body, "<title>Doc</title>") {
			t.Error("expected title 'Doc' in rendered output")
		}
	})

	t.Run("navbar is hidden when DisableNavBar is true", func(t *testing.T) {
		data := homeData{
			Page: pageData{Title: "Doc", DisableNavBar: true},
		}
		body := tr.render(t, "home", data)
		if strings.Contains(body, `class="navbar"`) {
			t.Error("expected navbar to be hidden when DisableNavBar is true")
		}
	})

	t.Run("dark mode class applied", func(t *testing.T) {
		data := homeData{
			Page: pageData{Title: "Doc", DisableNavBar: true, IsDarkMode: true},
		}
		body := tr.render(t, "home", data)
		if !strings.Contains(body, "dark-mode") {
			t.Error("expected dark-mode class in body tag")
		}
	})

	t.Run("analytics script included when enabled", func(t *testing.T) {
		data := homeData{
			Page: pageData{Title: "Doc", DisableNavBar: true, Analytics: true},
		}
		body := tr.render(t, "home", data)
		if !strings.Contains(body, "googletagmanager.com") {
			t.Error("expected analytics script when Analytics is true")
		}
	})

	t.Run("analytics script excluded when disabled", func(t *testing.T) {
		data := homeData{
			Page: pageData{Title: "Doc", DisableNavBar: true, Analytics: false},
		}
		body := tr.render(t, "home", data)
		if strings.Contains(body, "googletagmanager.com") {
			t.Error("expected no analytics script when Analytics is false")
		}
	})
}

func TestNewTemplate(t *testing.T) {
	tr := newTestRenderer(t)

	t.Run("renders with homeData (no Host)", func(t *testing.T) {
		data := homeData{
			Page: pageData{Title: "Indexing..."},
		}
		body := tr.render(t, "new", data)
		if !strings.Contains(body, "indexed") {
			t.Error("expected indexing message in rendered output")
		}
	})

	t.Run("no breadcrumbs on new page", func(t *testing.T) {
		// The "new" template has no navbar-new partial, so breadcrumbs never render.
		data := homeData{
			Page: pageData{Title: "Indexing..."},
		}
		body := tr.render(t, "new", data)
		if strings.Contains(body, "breadcrumb-item") {
			t.Error("expected no breadcrumb on new page (no navbar-new partial)")
		}
	})

	t.Run("renders with navbar when DisableNavBar is false", func(t *testing.T) {
		data := homeData{
			Page: pageData{Title: "Indexing..."},
		}
		body := tr.render(t, "new", data)
		if !strings.Contains(body, `class="navbar"`) {
			t.Error("expected navbar to be present when DisableNavBar is false")
		}
	})
}

func TestOrgTemplate(t *testing.T) {
	tr := newTestRenderer(t)

	basePage := pageData{
		Title: "crossplane/crossplane",
		Host:  "github.com",
		Repo:  "crossplane/crossplane",
		Tag:   "v1.0.0",
	}

	t.Run("renders without error", func(t *testing.T) {
		data := orgData{
			Page:  basePage,
			Tags:  []string{"v1.0.0", "v0.14.0"},
			CRDs:  map[string]models.RepoCRD{},
			Total: 0,
		}
		body := tr.render(t, "org", data)
		if !strings.Contains(body, "crossplane/crossplane") {
			t.Error("expected repo name in rendered output")
		}
	})

	t.Run("renders repo link with host and tag", func(t *testing.T) {
		data := orgData{
			Page:  basePage,
			Tags:  []string{"v1.0.0"},
			CRDs:  map[string]models.RepoCRD{},
			Total: 0,
		}
		body := tr.render(t, "org", data)
		if !strings.Contains(body, "/github.com/crossplane/crossplane@v1.0.0") {
			t.Error("expected repo link with host/repo@tag format")
		}
	})

	t.Run("renders tree link to source", func(t *testing.T) {
		data := orgData{
			Page:  basePage,
			Tags:  []string{"v1.0.0"},
			CRDs:  map[string]models.RepoCRD{},
			Total: 0,
		}
		body := tr.render(t, "org", data)
		if !strings.Contains(body, "https://github.com/crossplane/crossplane/tree/v1.0.0") {
			t.Error("expected GitHub tree link")
		}
	})

	t.Run("renders master tree link when tag is empty", func(t *testing.T) {
		noTagPage := basePage
		noTagPage.Tag = ""
		data := orgData{
			Page:  noTagPage,
			Tags:  []string{},
			CRDs:  map[string]models.RepoCRD{},
			Total: 0,
		}
		body := tr.render(t, "org", data)
		if !strings.Contains(body, "/tree/master") {
			t.Error("expected /tree/master link when tag is empty")
		}
	})

	t.Run("renders tag select options", func(t *testing.T) {
		data := orgData{
			Page:  basePage,
			Tags:  []string{"v1.0.0", "v0.14.0", "v0.13.0"},
			CRDs:  map[string]models.RepoCRD{},
			Total: 0,
		}
		body := tr.render(t, "org", data)
		if !strings.Contains(body, "v0.14.0") {
			t.Error("expected tag option v0.14.0")
		}
		if !strings.Contains(body, "selected=\"selected\"") {
			t.Error("expected a selected tag option")
		}
	})

	t.Run("renders CRD total count", func(t *testing.T) {
		data := orgData{
			Page: basePage,
			Tags: []string{"v1.0.0"},
			CRDs: map[string]models.RepoCRD{
				"cache.aws/v1alpha1/CacheCluster": {Group: "cache.aws", Version: "v1alpha1", Kind: "CacheCluster"},
			},
			Total: 1,
		}
		body := tr.render(t, "org", data)
		if !strings.Contains(body, "<b>1</b>") {
			t.Error("expected CRD count of 1")
		}
	})

	t.Run("no breadcrumbs on org page", func(t *testing.T) {
		// The org template has no navbar-org partial, so the {{ partial "navbar" }}
		// call in _navbar.html renders empty. Breadcrumbs only appear on the doc page.
		data := orgData{
			Page:  basePage,
			Tags:  []string{"v1.0.0"},
			CRDs:  map[string]models.RepoCRD{},
			Total: 0,
		}
		body := tr.render(t, "org", data)
		if strings.Contains(body, "breadcrumb-item") {
			t.Error("expected no breadcrumb on org page (no navbar-org partial)")
		}
	})

	t.Run("toJSON produces valid JSON with CRDs", func(t *testing.T) {
		data := orgData{
			Page: basePage,
			Tags: []string{"v1.0.0"},
			CRDs: map[string]models.RepoCRD{
				"cache.aws/v1alpha1/CacheCluster": {Group: "cache.aws", Version: "v1alpha1", Kind: "CacheCluster"},
			},
			Total: 1,
		}
		body := tr.render(t, "org", data)

		jsonStart := strings.Index(body, `const { Page, CRDs } = (`)
		if jsonStart == -1 {
			t.Fatal("could not find JSON assignment in org template output")
		}
		jsonPayload := body[jsonStart+len(`const { Page, CRDs } = (`):]
		jsonEnd := strings.Index(jsonPayload, ");")
		if jsonEnd == -1 {
			t.Fatal("could not find end of JSON assignment")
		}
		jsonPayload = strings.TrimSpace(jsonPayload[:jsonEnd])

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(jsonPayload), &parsed); err != nil {
			t.Fatalf("toJSON output is not valid JSON: %v\npayload: %s", err, jsonPayload)
		}
		if _, ok := parsed["Page"]; !ok {
			t.Error("expected 'Page' key in JSON output")
		}
		if _, ok := parsed["CRDs"]; !ok {
			t.Error("expected 'CRDs' key in JSON output")
		}
	})
}

func TestDocTemplate(t *testing.T) {
	tr := newTestRenderer(t)

	makeDocData := func() docData {
		return docData{
			Page: pageData{
				Title:   "CacheCluster.cache.aws/v1alpha1",
				Host:    "github.com",
				Repo:    "crossplane/provider-aws",
				Tag:     "v0.20.0",
				Group:   "cache.aws",
				Version: "v1alpha1",
				Kind:    "CacheCluster",
			},
			Description: "A managed resource that represents an AWS ElastiCache cluster.",
			Schema: apiextensions.JSONSchemaProps{
				Type:        "object",
				Description: "CacheCluster is the Schema for the cachecluster API.",
				Properties: map[string]apiextensions.JSONSchemaProps{
					"spec": {
						Type:        "object",
						Description: "CacheClusterSpec defines the desired state.",
						Properties: map[string]apiextensions.JSONSchemaProps{
							"region": {Type: "string", Description: "AWS region."},
						},
					},
					"status": {
						Type:        "object",
						Description: "CacheClusterStatus defines the observed state.",
					},
				},
			},
		}
	}

	t.Run("renders without error", func(t *testing.T) {
		body := tr.render(t, "doc", makeDocData())
		if !strings.Contains(body, "CacheCluster.cache.aws/v1alpha1") {
			t.Error("expected title in rendered output")
		}
	})

	t.Run("navbar shows breadcrumbs", func(t *testing.T) {
		body := tr.render(t, "doc", makeDocData())
		if !strings.Contains(body, "breadcrumb-item") {
			t.Error("expected breadcrumb items in navbar for doc page")
		}
		if !strings.Contains(body, "crossplane/provider-aws@v0.20.0") {
			t.Error("expected repo@tag in navbar breadcrumb")
		}
	})

	t.Run("toJSON produces valid JSON with Schema", func(t *testing.T) {
		body := tr.render(t, "doc", makeDocData())

		jsonStart := strings.Index(body, `<script id="pageData" type="application/json">`)
		if jsonStart == -1 {
			t.Fatal("could not find pageData script tag")
		}
		jsonPayload := body[jsonStart+len(`<script id="pageData" type="application/json">`):]
		jsonEnd := strings.Index(jsonPayload, "</script>")
		if jsonEnd == -1 {
			t.Fatal("could not find end of pageData script")
		}
		jsonPayload = strings.TrimSpace(jsonPayload[:jsonEnd])

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(jsonPayload), &parsed); err != nil {
			t.Fatalf("toJSON output is not valid JSON: %v\npayload (first 500 chars): %.500s", err, jsonPayload)
		}

		page, ok := parsed["Page"].(map[string]interface{})
		if !ok {
			t.Fatal("expected 'Page' object in JSON output")
		}
		if page["Kind"] != "CacheCluster" {
			t.Errorf("expected Kind=CacheCluster, got %v", page["Kind"])
		}
		if page["Group"] != "cache.aws" {
			t.Errorf("expected Group=cache.aws, got %v", page["Group"])
		}
		if page["Version"] != "v1alpha1" {
			t.Errorf("expected Version=v1alpha1, got %v", page["Version"])
		}

		schema, ok := parsed["Schema"].(map[string]interface{})
		if !ok {
			t.Fatal("expected 'Schema' object in JSON output")
		}
		if schema["Type"] != "object" {
			t.Errorf("expected Schema.Type=object, got %v", schema["Type"])
		}
	})

	t.Run("renders with empty schema properties", func(t *testing.T) {
		data := makeDocData()
		data.Schema = apiextensions.JSONSchemaProps{
			Type:        "object",
			Description: "A CRD with no properties.",
		}
		tr.render(t, "doc", data)
	})

	t.Run("renders with nested array schema", func(t *testing.T) {
		data := makeDocData()
		data.Schema = apiextensions.JSONSchemaProps{
			Type:        "object",
			Description: "A CRD with array properties.",
			Properties: map[string]apiextensions.JSONSchemaProps{
				"spec": {
					Type: "object",
					Properties: map[string]apiextensions.JSONSchemaProps{
						"items": {
							Type: "array",
							Items: &apiextensions.JSONSchemaPropsOrArray{
								Schema: &apiextensions.JSONSchemaProps{
									Type: "object",
									Properties: map[string]apiextensions.JSONSchemaProps{
										"name": {Type: "string"},
									},
								},
							},
						},
					},
				},
			},
		}
		tr.render(t, "doc", data)
	})
}

// --- Struct field compatibility tests ---

func TestNavbarPartialFieldCompatibility(t *testing.T) {
	tr := newTestRenderer(t)

	// The unrolled/render library's {{ partial "navbar" }} looks for a template
	// named "navbar-{current_template}". Only navbar-doc.html exists, so:
	// - home  → no navbar-home  → partial renders empty
	// - new   → no navbar-new   → partial renders empty
	// - org   → no navbar-org   → partial renders empty
	// - doc   → navbar-doc.html → breadcrumbs rendered (if .Page.Host is set)

	t.Run("homeData renders without crash", func(t *testing.T) {
		data := homeData{Page: pageData{Title: "Home", DisableNavBar: true}}
		tr.render(t, "home", data)
	})

	t.Run("new template renders without crash for all data types", func(t *testing.T) {
		data := homeData{Page: pageData{Title: "Indexing..."}}
		tr.render(t, "new", data)
	})

	t.Run("org template renders without crash", func(t *testing.T) {
		data := orgData{
			Page: pageData{Title: "test", Host: "github.com", Repo: "org/repo", Tag: "v1.0.0"},
			Tags: []string{"v1.0.0"}, CRDs: map[string]models.RepoCRD{}, Total: 0,
		}
		tr.render(t, "org", data)
	})

	t.Run("doc template shows breadcrumbs when Host is set", func(t *testing.T) {
		data := docData{
			Page: pageData{
				Title: "test", Host: "github.com", Repo: "org/repo", Tag: "v1.0.0",
				Kind: "MyKind", Version: "v1", Group: "test.io",
			},
			Schema: apiextensions.JSONSchemaProps{Type: "object"},
		}
		body := tr.render(t, "doc", data)
		if !strings.Contains(body, "MyKind.v1.test.io") {
			t.Error("expected Kind.Version.Group in navbar breadcrumb")
		}
	})

	t.Run("doc template hides breadcrumbs when Host is empty", func(t *testing.T) {
		data := docData{
			Page:   pageData{Title: "test"},
			Schema: apiextensions.JSONSchemaProps{Type: "object"},
		}
		body := tr.render(t, "doc", data)
		if strings.Contains(body, "breadcrumb-item") {
			t.Error("expected no breadcrumbs when Host is empty")
		}
	})
}

// --- CRD link URL format test ---

func TestOrgCRDLinkURLFormat(t *testing.T) {
	tr := newTestRenderer(t)

	data := orgData{
		Page: pageData{
			Title: "test",
			Host:  "github.com",
			Repo:  "crossplane/crossplane",
			Tag:   "v1.0.0",
		},
		Tags: []string{"v1.0.0"},
		CRDs: map[string]models.RepoCRD{
			"cache.aws/v1alpha1/CacheCluster": {Group: "cache.aws", Version: "v1alpha1", Kind: "CacheCluster"},
		},
		Total: 1,
	}
	body := tr.render(t, "org", data)

	// The URL in the JS should be /{Host}/{Repo}/{Group}/{Version}/{Kind}@{Tag}
	// which matches parseRepoURL's expected format.
	expectedURLPattern := `/${Host}/${Repo}/${original.Group}/${original.Version}/${original.Kind}@${Tag}`
	if !strings.Contains(body, expectedURLPattern) {
		t.Errorf("expected CRD link URL pattern %q in rendered JS", expectedURLPattern)
	}
}

// --- getPageData tests ---

func TestGetPageData(t *testing.T) {
	t.Run("sets title", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		pd := getPageData(r, "My Title", false)
		if pd.Title != "My Title" {
			t.Errorf("expected Title='My Title', got %q", pd.Title)
		}
	})

	t.Run("sets DisableNavBar", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		pd := getPageData(r, "test", true)
		if !pd.DisableNavBar {
			t.Error("expected DisableNavBar=true")
		}
	})

	t.Run("dark mode from cookie", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.AddCookie(&http.Cookie{Name: cookieDarkMode, Value: "dark-mode"})
		pd := getPageData(r, "test", false)
		if !pd.IsDarkMode {
			t.Error("expected IsDarkMode=true when cookie is set")
		}
	})

	t.Run("no dark mode without cookie", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		pd := getPageData(r, "test", false)
		if pd.IsDarkMode {
			t.Error("expected IsDarkMode=false without cookie")
		}
	})

	t.Run("shared fields default to zero values", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		pd := getPageData(r, "test", false)
		if pd.Host != "" || pd.Repo != "" || pd.Tag != "" || pd.Group != "" || pd.Version != "" || pd.Kind != "" {
			t.Error("expected all route context fields to be empty by default")
		}
	})
}

// --- tryIndex tests ---

func TestTryIndex(t *testing.T) {
	t.Run("returns true when channel has capacity", func(t *testing.T) {
		ch := make(chan models.GitterRepo, 1)
		ok := tryIndex(models.GitterRepo{Host: "github.com", Org: "org", Repo: "repo"}, ch)
		if !ok {
			t.Error("expected tryIndex to return true with available capacity")
		}
	})

	t.Run("returns false when channel is full", func(t *testing.T) {
		ch := make(chan models.GitterRepo, 1)
		ch <- models.GitterRepo{}
		ok := tryIndex(models.GitterRepo{Host: "github.com", Org: "org", Repo: "repo"}, ch)
		if ok {
			t.Error("expected tryIndex to return false when channel is full")
		}
	})
}
