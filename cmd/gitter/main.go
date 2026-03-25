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

// Package main provides a standalone RPC gitter service for backward
// compatibility. New deployments should use the merged doc binary with
// DB_DRIVER=sqlite, which indexes directly without RPC.
package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"

	"github.com/crdsdev/doc/pkg/indexer"
	"github.com/crdsdev/doc/pkg/models"
	"github.com/crdsdev/doc/pkg/provider"
	"github.com/crdsdev/doc/pkg/store"
)

func main() {
	s, err := store.NewFromEnv()
	if err != nil {
		panic(err)
	}
	defer s.Close()

	reg, err := provider.RegistryFromConfigs(provider.LoadConfigFile(os.Getenv("CONFIG_FILE")), os.Getenv)
	if err != nil {
		panic(err)
	}

	gitter := &Gitter{idx: indexer.New(s, reg)}
	rpc.Register(gitter)
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", ":1234")
	if e != nil {
		log.Fatal("listen error:", e)
	}
	log.Println("Starting gitter (standalone, backward-compat mode)...")
	http.Serve(l, nil)
}

// Gitter wraps the indexer for net/rpc compatibility.
type Gitter struct {
	idx *indexer.Indexer
}

// Index indexes a git repo at the specified url.
func (g *Gitter) Index(gRepo models.GitterRepo, reply *string) error {
	return g.idx.Index(context.Background(), gRepo)
}
