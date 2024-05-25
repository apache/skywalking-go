// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Apache Software Foundation (ASF) licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"context"

	"log"
	"net/http"
	"strconv"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	_ "github.com/apache/skywalking-go"
)

type testFunc func() error

var (
	client    *elasticsearch.TypedClient
	url       = "http://elasticsearch:9200"
	ctx       = context.Background()
	indexName = "sw-index"
)

type SwDoc struct {
	ID     int64  `json:"id"`
	Simple string `json:"simple"`
	Love   string `json:"love"`
}

func main() {
	cfg := elasticsearch.Config{
		Addresses: []string{url},
	}
	var err error
	client, err = elasticsearch.NewTypedClient(cfg)
	if err != nil {
		log.Fatalf("connect to elasticsearch error: %v \n", err)
	}
	http.HandleFunc("/execute", executeHandler)
	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})
	_ = http.ListenAndServe(":8080", nil)
}

func executeHandler(w http.ResponseWriter, r *http.Request) {
	testCases := []struct {
		name string
		fn   testFunc
	}{
		{"createIndex", createIndex},
		{"indexDocument", indexDocument},
		{"getDocument", getDocument},
		{"searchDocument", searchDocument},
	}

	for _, test := range testCases {
		log.Printf("excute test case %s", test.name)
		if err := test.fn(); err != nil {
			log.Fatalf("test case %s failed: %v", test.name, err)
		}
	}
}

func createIndex() error {
	_, err := client.Indices.
		Create(indexName).
		Do(ctx)
	if err != nil {
		log.Fatalf("create index failed, err:%v\n", err)
		return err
	}
	return nil
}

func indexDocument() error {
	simpleLove := SwDoc{
		ID:     1,
		Simple: "Just Simple",
		Love:   "Just Love",
	}
	_, err := client.Index(indexName).
		Id(strconv.FormatInt(simpleLove.ID, 10)).
		Document(simpleLove).
		Do(ctx)
	if err != nil {
		log.Fatalf("indexing document failed, err:%v\n", err)
		return err
	}
	return nil
}

func getDocument() error {
	_, err := client.Get(indexName, "1").
		Do(context.Background())
	if err != nil {
		log.Printf("get document by id failed, err:%v\n", err)
		return err
	}
	return nil
}

func searchDocument() error {
	_, err := client.Search().
		Index(indexName).
		Request(&search.Request{
			Query: &types.Query{
				Match: map[string]types.MatchQuery{
					"love": {Query: "Just Love"},
				},
			},
		}).Do(ctx)
	if err != nil {
		log.Printf("search document failed, err:%v\n", err)
		return err
	}
	return nil
}
