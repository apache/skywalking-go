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
	"fmt"
	"log"
	"net/http"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	_ "github.com/apache/skywalking-go"
)

var db *gorm.DB

type testFunc func(*gorm.DB) error

type User struct {
	ID   uint
	Name string
	Age  uint8
}

func executeHandler(w http.ResponseWriter, r *http.Request) {
	tests := []struct {
		name string
		fn   testFunc
	}{
		{"raw", TestRaw},
		{"create", TestCreate},
		{"query", TestQuery},
		{"row", TestRow},
		{"update", TestUpdate},
		{"delete", TestDelete},
	}

	dbWithCtx := db.WithContext(r.Context())
	for _, test := range tests {
		log.Printf("excute test case %s", test.name)
		if err := test.fn(dbWithCtx); err != nil {
			log.Fatalf("test case %s failed: %v", test.name, err)
		}
	}
	_, _ = w.Write([]byte("execute sql success"))
}

func TestRaw(db *gorm.DB) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS users (id char(255), name VARCHAR(255), age INTEGER)`).Error; err != nil {
		return fmt.Errorf("create error: %s", err.Error())
	}

	return nil
}

func TestCreate(db *gorm.DB) error {
	user := User{Name: "Jinzhu", Age: 18}
	if err := db.Create(&user).Error; err != nil {
		return fmt.Errorf("create error: %w", err)
	}

	return nil
}

func TestQuery(db *gorm.DB) error {
	var user User
	if err := db.First(&user).Error; err != nil {
		return fmt.Errorf("query error: %w", err)
	}

	return nil
}

func TestRow(db *gorm.DB) error {
	var name string
	var age uint8
	row := db.Table("users").Where("name = ?", "jinzhu").Select("name", "age").Row()
	row.Scan(&name, &age)

	return nil
}

func TestUpdate(db *gorm.DB) error {
	tx := db.Model(&User{}).Where("name = ?", "jinzhu").Update("name", "hello")
	if err := tx.Error; err != nil {
		return fmt.Errorf("update error: %w", err)
	}

	return nil
}

func TestDelete(db *gorm.DB) error {
	if err := db.Delete(&User{}, 1).Error; err != nil {
		return fmt.Errorf("delete error: %w", err)
	}

	return nil
}

func main() {
	tmpDB, err := gorm.Open(mysql.Open("root:root@tcp(mysql-server:3306)/test"), &gorm.Config{})
	if err != nil {
		log.Fatalf("open db error: %v \n", err)
	}
	db = tmpDB

	http.HandleFunc("/execute", executeHandler)

	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})

	_ = http.ListenAndServe(":8080", nil)
}
