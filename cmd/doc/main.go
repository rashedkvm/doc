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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	crdutil "github.com/crdsdev/doc/pkg/crd"
	"github.com/crdsdev/doc/pkg/indexer"
	"github.com/crdsdev/doc/pkg/models"
	"github.com/crdsdev/doc/pkg/provider"
	"github.com/crdsdev/doc/pkg/store"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	flag "github.com/spf13/pflag"
	"github.com/unrolled/render"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

var (
	db  store.Store
	idx *indexer.Indexer
	reg *provider.Registry
)

var (
	envAnalytics   = "ANALYTICS"
	envDevelopment = "IS_DEV"

	cookieDarkMode = "halfmoon_preferredMode"

	analytics bool = false

	gitterChan chan models.GitterRepo
)

// SchemaPlusParent is a JSON schema plus the name of the parent field.
type SchemaPlusParent struct {
	Parent string
	Schema map[string]apiextensions.JSONSchemaProps
}

var page = render.New(render.Options{
	Extensions:    []string{".html"},
	Directory:     "template",
	Layout:        "layout",
	IsDevelopment: os.Getenv(envDevelopment) == "true",
	Funcs: []template.FuncMap{
		{
			"plusParent": func(p string, s map[string]apiextensions.JSONSchemaProps) *SchemaPlusParent {
				return &SchemaPlusParent{
					Parent: p,
					Schema: s,
				}
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

type pageData struct {
	Analytics     bool
	DisableNavBar bool
	IsDarkMode    bool
	Title         string
	Host          string
	Repo          string
	Tag           string
	At            string
	Group         string
	Version       string
	Kind          string
}

type docData struct {
	Page        pageData
	Description string
	Schema      apiextensions.JSONSchemaProps
}

type orgData struct {
	Page  pageData
	Tags  []string
	CRDs  map[string]models.RepoCRD
	Total int
}

type homeData struct {
	Page  pageData
	Repos []string
}

func worker(gitterChan <-chan models.GitterRepo) {
	for job := range gitterChan {
		if err := idx.Index(context.Background(), job); err != nil {
			log.Printf("indexing %s/%s: %v", job.Org, job.Repo, err)
		}
	}
}

func tryIndex(repo models.GitterRepo, gitterChan chan models.GitterRepo) bool {
	select {
	case gitterChan <- repo:
		return true
	default:
		return false
	}
}

func init() {
	analyticsStr := os.Getenv(envAnalytics)
	if analyticsStr == "true" {
		analytics = true
	}

	gitterChan = make(chan models.GitterRepo, 4)
}

func main() {
	flag.Parse()

	var err error
	db, err = store.NewFromEnv()
	if err != nil {
		panic(err)
	}
	defer db.Close()

	reg, err = provider.RegistryFromConfigs(provider.LoadConfigFile(os.Getenv("CONFIG_FILE")), os.Getenv)
	if err != nil {
		panic(err)
	}

	idx = indexer.New(db, reg)

	for i := 0; i < 4; i++ {
		go worker(gitterChan)
	}

	start()
}

func getPageData(r *http.Request, title string, disableNavBar bool) pageData {
	var isDarkMode = false
	if cookie, err := r.Cookie(cookieDarkMode); err == nil && cookie.Value == "dark-mode" {
		isDarkMode = true
	}
	return pageData{
		Analytics:     analytics,
		IsDarkMode:    isDarkMode,
		DisableNavBar: disableNavBar,
		Title:         title,
	}
}

func start() {
	log.Println("Starting Doc server...")
	r := mux.NewRouter().StrictSlash(true)
	staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("./static/")))
	r.HandleFunc("/", home)
	r.PathPrefix("/static/").Handler(staticHandler)
	r.HandleFunc("/{host}/{org}/{repo}@{tag}", orgHandler)
	r.HandleFunc("/{host}/{org}/{repo}", orgHandler)
	r.HandleFunc("/raw/{host}/{org}/{repo}@{tag}", raw)
	r.HandleFunc("/raw/{host}/{org}/{repo}", raw)
	r.PathPrefix("/").HandlerFunc(doc)
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "5000"
	}
	log.Printf("Listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func home(w http.ResponseWriter, r *http.Request) {
	data := homeData{Page: getPageData(r, "Doc", true)}
	if err := page.HTML(w, http.StatusOK, "home", data); err != nil {
		log.Printf("homeTemplate.Execute(): %v", err)
		fmt.Fprint(w, "Unable to render home template.")
		return
	}
	log.Print("successfully rendered home page")
}

func raw(w http.ResponseWriter, r *http.Request) {
	parameters := mux.Vars(r)
	host := parameters["host"]
	orgParam := parameters["org"]
	repoParam := parameters["repo"]
	tag := parameters["tag"]

	if _, err := reg.Resolve(host); err != nil {
		http.Error(w, "Unknown repository host.", http.StatusBadRequest)
		return
	}

	fullRepo := fmt.Sprintf("%s/%s/%s", host, orgParam, repoParam)

	rawCRDs, err := db.GetRawCRDs(r.Context(), fullRepo, tag)
	if err != nil {
		fmt.Fprint(w, "Unable to render raw CRDs.")
		log.Printf("failed to get raw CRDs for %s : %v", repoParam, err)
		return
	}

	var total []byte
	for _, res := range rawCRDs {
		crd := &apiextensions.CustomResourceDefinition{}
		if err := yaml.Unmarshal(res, crd); err != nil {
			continue
		}
		crdv1 := &v1.CustomResourceDefinition{}
		if err := v1.Convert_apiextensions_CustomResourceDefinition_To_v1_CustomResourceDefinition(crd, crdv1, nil); err != nil {
			continue
		}
		crdv1.SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("CustomResourceDefinition"))
		y, err := yaml.Marshal(crdv1)
		if err != nil {
			continue
		}
		total = append(total, y...)
		total = append(total, []byte("\n---\n")...)
	}

	w.Write(total)
	log.Printf("successfully rendered raw CRDs")

	if analytics {
		sendAnalytics(r)
	}
}

func orgHandler(w http.ResponseWriter, r *http.Request) {
	parameters := mux.Vars(r)
	host := parameters["host"]
	orgParam := parameters["org"]
	repoParam := parameters["repo"]
	tag := parameters["tag"]

	if _, err := reg.Resolve(host); err != nil {
		http.Error(w, "Unknown repository host.", http.StatusBadRequest)
		return
	}

	pd := getPageData(r, fmt.Sprintf("%s/%s", orgParam, repoParam), false)
	fullRepo := fmt.Sprintf("%s/%s/%s", host, orgParam, repoParam)

	if tag != "" {
		pd.Title += fmt.Sprintf("@%s", tag)
	}

	crdRows, foundTag, err := db.GetCRDsForRepo(r.Context(), fullRepo, tag)
	if err != nil {
		log.Printf("failed to get CRDs for %s : %v", repoParam, err)
		if err := page.HTML(w, http.StatusOK, "new", homeData{Page: pd}); err != nil {
			log.Printf("newTemplate.Execute(): %v", err)
			fmt.Fprint(w, "Unable to render new template.")
		}
		return
	}

	repoCRDs := map[string]models.RepoCRD{}
	for _, cr := range crdRows {
		repoCRDs[cr.Group+"/"+cr.Version+"/"+cr.Kind] = models.RepoCRD{
			Group:   cr.Group,
			Version: cr.Version,
			Kind:    cr.Kind,
		}
	}

	tags, err := db.GetTags(r.Context(), fullRepo)
	if err != nil {
		log.Printf("failed to get tags for %s : %v", repoParam, err)
		if err := page.HTML(w, http.StatusOK, "new", homeData{Page: pd}); err != nil {
			log.Printf("newTemplate.Execute(): %v", err)
			fmt.Fprint(w, "Unable to render new template.")
		}
		return
	}

	tagExists := false
	for _, t := range tags {
		if t == tag {
			tagExists = true
			break
		}
	}

	if len(tags) == 0 || (!tagExists && tag != "") {
		tryIndex(models.GitterRepo{
			Host: host,
			Org:  orgParam,
			Repo: repoParam,
			Tag:  tag,
		}, gitterChan)
		if err := page.HTML(w, http.StatusOK, "new", homeData{Page: pd}); err != nil {
			log.Printf("newTemplate.Execute(): %v", err)
			fmt.Fprint(w, "Unable to render new template.")
		}
		return
	}

	if foundTag == "" && len(tags) > 0 {
		foundTag = tags[0]
	}

	pd.Host = host
	pd.Repo = strings.Join([]string{orgParam, repoParam}, "/")
	pd.Tag = foundTag

	if err := page.HTML(w, http.StatusOK, "org", orgData{
		Page:  pd,
		Tags:  tags,
		CRDs:  repoCRDs,
		Total: len(repoCRDs),
	}); err != nil {
		log.Printf("orgTemplate.Execute(): %v", err)
		fmt.Fprint(w, "Unable to render org template.")
		return
	}
	log.Printf("successfully rendered org template")
}

func doc(w http.ResponseWriter, r *http.Request) {
	var schema *apiextensions.CustomResourceValidation
	log.Printf("Request Received: %s\n", r.URL.Path)
	host, orgParam, repoParam, group, version, kind, tag, err := parseRepoURL(r.URL.Path)
	if err != nil {
		log.Printf("failed to parse repo path: %v", err)
		fmt.Fprint(w, "Invalid URL.")
		return
	}
	if _, err := reg.Resolve(host); err != nil {
		log.Printf("unknown host in URL: %v", err)
		fmt.Fprint(w, "Unknown repository host.")
		return
	}
	pd := getPageData(r, fmt.Sprintf("%s.%s/%s", kind, group, version), false)
	fullRepo := fmt.Sprintf("%s/%s/%s", host, orgParam, repoParam)

	data, foundTag, err := db.GetCRD(r.Context(), fullRepo, tag, group, version, kind)
	if err != nil {
		log.Printf("failed to get CRDs for %s : %v", repoParam, err)
		if err := page.HTML(w, http.StatusOK, "new", homeData{Page: pd}); err != nil {
			log.Printf("newTemplate.Execute(): %v", err)
			fmt.Fprint(w, "Unable to render new template.")
		}
		return
	}

	crd := &apiextensions.CustomResourceDefinition{}
	if err := yaml.Unmarshal(data, crd); err != nil {
		log.Printf("failed to unmarshal CRD: %v", err)
		fmt.Fprint(w, "Unable to parse CRD data.")
		return
	}

	schema = crd.Spec.Validation
	if len(crd.Spec.Versions) > 1 {
		for _, version := range crd.Spec.Versions {
			if version.Storage {
				if version.Schema != nil {
					schema = version.Schema
				}
				break
			}
		}
	}

	if schema == nil || schema.OpenAPIV3Schema == nil {
		log.Print("CRD schema is nil.")
		fmt.Fprint(w, "Supplied CRD has no schema.")
		return
	}

	gvk := crdutil.GetStoredGVK(crd)
	if gvk == nil {
		log.Print("CRD GVK is nil.")
		fmt.Fprint(w, "Supplied CRD has no GVK.")
		return
	}

	pd.Host = host
	pd.Repo = strings.Join([]string{orgParam, repoParam}, "/")
	pd.Tag = foundTag
	pd.Group = gvk.Group
	pd.Version = gvk.Version
	pd.Kind = gvk.Kind

	if err := page.HTML(w, http.StatusOK, "doc", docData{
		Page:        pd,
		Description: string(schema.OpenAPIV3Schema.Description),
		Schema:      *schema.OpenAPIV3Schema,
	}); err != nil {
		log.Printf("docTemplate.Execute(): %v", err)
		fmt.Fprint(w, "Supplied CRD has no schema.")
		return
	}
	log.Printf("successfully rendered doc template")
}

// parseRepoURL parses a URL path of the form:
//
//	/{host}/{org}/{repo}/{group}/{version}/{kind}[@{tag}]
//
// It is host-agnostic — the first path element is treated as the host.
func parseRepoURL(uPath string) (host, org, repo, group, version, kind, tag string, err error) {
	u, err := url.Parse(uPath)
	if err != nil {
		return "", "", "", "", "", "", "", err
	}
	elements := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(elements) < 6 {
		return "", "", "", "", "", "", "", errors.New("invalid path: need at least /{host}/{org}/{repo}/{group}/{version}/{kind}")
	}

	tagSplit := strings.Split(u.Path, "@")
	if len(tagSplit) > 1 {
		tag = tagSplit[1]
	}

	return elements[0], elements[1], elements[2], elements[3], elements[4], strings.Split(elements[5], "@")[0], tag, nil
}

func sendAnalytics(r *http.Request) {
	u := uuid.New().String()
	metrics := url.Values{
		"v":   {"1"},
		"t":   {"pageview"},
		"tid": {"UA-116820283-2"},
		"cid": {u},
		"dh":  {"doc.crds.dev"},
		"dp":  {r.URL.Path},
		"uip": {r.RemoteAddr},
	}
	client := &http.Client{}

	req, _ := http.NewRequest("POST", "http://www.google-analytics.com/collect", strings.NewReader(metrics.Encode()))
	req.Header.Add("User-Agent", r.UserAgent())
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	if _, err := client.Do(req); err != nil {
		log.Printf("failed to report analytics: %s", err.Error())
	} else {
		log.Printf("successfully reported analytics")
	}
}
