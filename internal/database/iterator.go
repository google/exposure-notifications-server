// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package database

import (
	pgx "github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type rowIterator struct {
	conn *pgxpool.Conn
	rows pgx.Rows
}

func (i *rowIterator) next() (done bool, err error) {
	if i.rows == nil {
		return true, nil
	}
	if !i.rows.Next() {
		return true, i.rows.Err()
	}
	return false, nil
}

func (i *rowIterator) close() error {
	defer i.conn.Release()
	if i.rows != nil {
		i.rows.Close()
		return i.rows.Err()
	}
	return nil
}
