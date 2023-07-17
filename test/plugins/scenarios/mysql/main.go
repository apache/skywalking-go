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
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"

	_ "github.com/apache/skywalking-go"
)

type testFunc func(context.Context, *sql.DB) error

func main() {
	db, err := sql.Open("mysql", "user:password@tcp(mysql:3306)/database")
	if err != nil {
		panic(err)
	}
	// See "Important settings" section.
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	route := http.NewServeMux()
	route.HandleFunc("/execute", func(res http.ResponseWriter, req *http.Request) {
		tests := []struct {
			name string
			fn   testFunc
		}{
			{"exec", testExec},
			{"stmt", testStmt},
			{"commitTx", testCommitTx},
			{"rollbackTx", testRollbackTx},
		}

		for _, test := range tests {
			log.Printf("excute test case %s", test.name)
			if err1 := test.fn(req.Context(), db); err1 != nil {
				log.Printf("test case %s failed: %v", test.name, err1)
			}
		}
		_, _ = res.Write([]byte("execute sql success"))
	})

	route.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("ok"))
	})

	log.Println("start client")
	err = http.ListenAndServe(":8080", route)
	if err != nil {
		log.Fatalf("client start error: %v \n", err)
	}

}

func testExec(ctx context.Context, db *sql.DB) error {
	if err := db.PingContext(ctx); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS users`); err != nil {
		return fmt.Errorf("exec drop error: %w", err)
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS users (id char(255), name VARCHAR(255), age INTEGER)`); err != nil {
		return fmt.Errorf("exec create error: %w", err)
	}

	// test insert
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, name, age) VALUE ( ?, ?, ?)`, "0", "foo", 10); err != nil {
		return fmt.Errorf("exec insert error: %w", err)
	}

	var name string
	// test select
	if err := db.QueryRowContext(ctx, `SELECT name FROM users WHERE id = ?`, "0").Scan(&name); err != nil {
		return fmt.Errorf("query select error: %w", err)
	}

	return nil
}

func testStmt(ctx context.Context, db *sql.DB) error {
	stmt, err := db.PrepareContext(ctx, `INSERT INTO users (id, name, age) VALUE ( ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() {
		_ = stmt.Close()
	}()

	_, err = stmt.ExecContext(ctx, "1", "bar", 11)
	if err != nil {
		return err
	}

	return nil
}

func testCommitTx(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx error: %v", err)
	}

	if _, err := tx.Exec(`INSERT INTO users (id, name, age) VALUE ( ?, ?, ? )`, "2", "foobar", 24); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE users SET name = ? WHERE id = ?`, "foobar", "0"); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func testRollbackTx(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx error: %v", err)
	}

	if _, err := tx.Exec(`UPDATE users SET age = ? WHERE id = ?`, 48, "2"); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE users SET name = ? WHERE id = ?`, "foobar", "1"); err != nil {
		return err
	}

	if err := tx.Rollback(); err != nil {
		return err
	}
	return nil
}
