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
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/crdsdev/doc/pkg/models"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	flag "github.com/spf13/pflag"
	"github.com/unrolled/render"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

var redisClient *redis.Client

// redis connection
var (
	envAddress     = "REDIS_HOST"
	envAnalytics   = "ANALYTICS"
	envDevelopment = "IS_DEV"

	cookieDarkMode = "halfmoon_preferredMode"

	address   string
	analytics bool = false
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
	IsDevelopment: os.Getenv(envAnalytics) == "true",
	Funcs: []template.FuncMap{
		{
			"plusParent": func(p string, s map[string]apiextensions.JSONSchemaProps) *SchemaPlusParent {
				return &SchemaPlusParent{
					Parent: p,
					Schema: s,
				}
			},
		},
	},
})

type pageData struct {
	Analytics     bool
	DisableNavBar bool
	IsDarkMode    bool
}

type baseData struct {
	Page pageData
}

type docData struct {
	Page        pageData
	Repo        string
	Tag         string
	At          string
	Group       string
	Version     string
	Kind        string
	Description string
	Schema      apiextensions.JSONSchemaProps
}

type orgData struct {
	Page       pageData
	Repo       string
	Tag        string
	At         string
	CRDs       map[string]models.RepoCRD
	Total      int
	LastParsed string
}

type homeData struct {
	Page  pageData
	Repos []string
}

func init() {
	address = os.Getenv(envAddress)

	// TODO(hasheddan): use a flag
	analyticsStr := os.Getenv(envAnalytics)
	if analyticsStr == "true" {
		analytics = true
	}
}

func main() {
	flag.Parse()

	redisClient = redis.NewClient(&redis.Options{
		Addr: address + ":6379",
	})
	start()
}

func getPageData(r *http.Request, disableNavBar bool) pageData {
	var isDarkMode = false
	if cookie, err := r.Cookie(cookieDarkMode); err == nil && cookie.Value == "dark-mode" {
		isDarkMode = true
	}
	return pageData{
		Analytics:     analytics,
		IsDarkMode:    isDarkMode,
		DisableNavBar: disableNavBar,
	}
}

func start() {
	log.Println("Starting Doc server...")
	r := mux.NewRouter().StrictSlash(true)
	staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("./static/")))
	r.HandleFunc("/", home)
	r.PathPrefix("/static/").Handler(staticHandler)
	r.HandleFunc("/github.com/{org}/{repo}@{tag}", org)
	r.HandleFunc("/github.com/{org}/{repo}", org)
	r.HandleFunc("/raw/github.com/{org}/{repo}@{tag}", raw)
	r.HandleFunc("/raw/github.com/{org}/{repo}", raw)
	r.PathPrefix("/").HandlerFunc(doc)
	log.Fatal(http.ListenAndServe(":5000", r))
}

func home(w http.ResponseWriter, r *http.Request) {
	data := homeData{Page: getPageData(r, true)}
	if res, err := redisClient.SMembers("repos:popular").Result(); err != nil {
		log.Printf("failed to get popular repos : %v", err)
	} else {
		data.Repos = res
	}
	if err := page.HTML(w, http.StatusOK, "home", data); err != nil {
		log.Printf("homeTemplate.Execute(): %v", err)
		fmt.Fprint(w, "Unable to render home template.")
		return
	}
	log.Print("successfully rendered home page")
}

func raw(w http.ResponseWriter, r *http.Request) {
	parameters := mux.Vars(r)
	org := parameters["org"]
	repo := parameters["repo"]
	tag := parameters["tag"]
	at := ""
	if tag != "" {
		at = "@"
	}
	res, err := redisClient.Get(strings.Join([]string{"raw/github.com", org, repo}, "/") + at + tag).Result()
	if err != nil {
		log.Printf("failed to get raw CRDs for %s : %v", repo, err)
		fmt.Fprint(w, "Unable to render raw CRDs.")
	}

	w.Write([]byte(res))
	log.Printf("successfully rendered raw CRDs")

	if analytics {
		u := uuid.New().String()
		// TODO(hasheddan): do not hardcode tid and dh
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
}

func org(w http.ResponseWriter, r *http.Request) {
	parameters := mux.Vars(r)
	org := parameters["org"]
	repo := parameters["repo"]
	tag := parameters["tag"]
	at := ""
	if tag != "" {
		at = "@"
	}
	res, err := redisClient.Get(strings.Join([]string{"github.com", org, repo}, "/") + at + tag).Result()
	if err != nil {
		log.Printf("failed to get CRDs for %s : %v", repo, err)
		if err := page.HTML(w, http.StatusOK, "new", baseData{Page: getPageData(r, false)}); err != nil {
			log.Printf("newTemplate.Execute(): %v", err)
			fmt.Fprint(w, "Unable to render new template.")
		}
		return
	}

	repoData := &models.Repo{}
	bytes := []byte(res)
	if err := json.Unmarshal(bytes, repoData); err != nil {
		log.Printf("failed to get CRDs for %s : %v", repo, err)
		page.HTML(w, http.StatusOK, "home", homeData{Page: getPageData(r, false)})
		return
	}
	if err := page.HTML(w, http.StatusOK, "org", orgData{
		Page:       getPageData(r, false),
		Repo:       strings.Join([]string{org, repo}, "/"),
		Tag:        tag,
		At:         at,
		CRDs:       repoData.CRDs,
		Total:      len(repoData.CRDs),
		LastParsed: repoData.LastParsed.Format(time.RFC1123Z),
	}); err != nil {
		log.Printf("orgTemplate.Execute(): %v", err)
		fmt.Fprint(w, "Unable to render org template.")
		return
	}
	log.Printf("successfully rendered org template")
}

func doc(w http.ResponseWriter, r *http.Request) {
	var schema *apiextensions.CustomResourceValidation
	crd := &apiextensions.CustomResourceDefinition{}
	log.Printf("Request Received: %s\n", r.URL.Path)
	org, repo, tag, err := parseGHURL(r.URL.Path)
	if err != nil {
		log.Printf("failed to parse Github path: %v", err)
		fmt.Fprint(w, "Invalid URL.")
		return
	}
	at := ""
	if tag != "" {
		at = "@"
	}
	res, err := redisClient.Get(strings.Trim(r.URL.Path, "/")).Result()
	if err != nil {
		log.Printf("failed to get CRDs for %s : %v", repo, err)
		if err := page.HTML(w, http.StatusOK, "doc", baseData{Page: getPageData(r, false)}); err != nil {
			log.Printf("newTemplate.Execute(): %v", err)
			fmt.Fprint(w, "Unable to render new template.")
		}
		return
	}

	if err := json.Unmarshal([]byte(res), crd); err != nil {
		log.Printf("failed to convert to CRD: %v", err)
		fmt.Fprint(w, "Supplied file is not a valid CRD.")
		return
	}

	schema = crd.Spec.Validation
	if len(crd.Spec.Versions) > 1 {
		for _, version := range crd.Spec.Versions {
			if version.Storage == true {
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

	if err := page.HTML(w, http.StatusOK, "doc", docData{
		Page:        getPageData(r, false),
		Repo:        strings.Join([]string{org, repo}, "/"),
		Tag:         tag,
		At:          at,
		Group:       crd.Spec.Group,
		Version:     crd.Spec.Version,
		Kind:        crd.Spec.Names.Kind,
		Description: string(schema.OpenAPIV3Schema.Description),
		Schema:      *schema.OpenAPIV3Schema,
	}); err != nil {
		log.Printf("docTemplate.Execute(): %v", err)
		fmt.Fprint(w, "Supplied CRD has no schema.")
		return
	}
	log.Printf("successfully rendered doc template")
}

// TODO(hasheddan): add testing and more reliable parse
func parseGHURL(uPath string) (org, repo, tag string, err error) {
	u, err := url.Parse(uPath)
	if err != nil {
		return "", "", "", err
	}
	elements := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(elements) < 4 {
		return "", "", "", errors.New("invalid path")
	}

	tagSplit := strings.Split(u.Path, "@")
	if len(tagSplit) > 1 {
		tag = tagSplit[1]
	}

	return elements[1], elements[2], tag, nil
}
