// Copyright 2016 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.
//
// Author: Radu Berinde (radu@cockroachlabs.com)

package sqlutils

import (
	"bytes"
	gosql "database/sql"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/sql/parser"
)

const rowsPerInsert = 100

// GenRowFn is a function that takes a (1-based) row index and returns a row of
// Datums that will be converted to strings to form part of an INSERT statement.
type GenRowFn func(row int) []parser.Datum

// genValues writes a string of generated values "(a,b,c),(d,e,f)...".
func genValues(w io.Writer, firstRow, lastRow int, fn GenRowFn) {
	for rowIdx := firstRow; rowIdx <= lastRow; rowIdx++ {
		if rowIdx > firstRow {
			fmt.Fprint(w, ",")
		}
		row := fn(rowIdx)
		fmt.Fprintf(w, "(%s", row[0])
		for _, v := range row[1:] {
			fmt.Fprintf(w, ",%s", v)
		}
		fmt.Fprint(w, ")")
	}
}

// CreateTable creates a table in the "test" database with the given number of
// rows and using the given row generation function.
func CreateTable(t *testing.T, sqlDB *gosql.DB, tableName, schema string, numRows int, fn GenRowFn) {
	r := MakeSQLRunner(t, sqlDB)
	stmt := `CREATE DATABASE IF NOT EXISTS test;`
	stmt += fmt.Sprintf(`CREATE TABLE test.%s (%s);`, tableName, schema)
	r.Exec(stmt)
	for i := 1; i <= numRows; {
		var buf bytes.Buffer
		fmt.Fprintf(&buf, `INSERT INTO test.%s VALUES `, tableName)
		batchEnd := i + rowsPerInsert
		if batchEnd > numRows {
			batchEnd = numRows
		}
		genValues(&buf, i, batchEnd, fn)
		r.Exec(buf.String())
		i = batchEnd + 1
	}
}

// GenValueFn is a function that takes a (1-based) row index and returns a Datum
// which will be converted to a string to form part of an INSERT statement.
type GenValueFn func(row int) parser.Datum

// RowIdxFn is a GenValueFn that returns the row number as a DInt
func RowIdxFn(row int) parser.Datum {
	return parser.NewDInt(parser.DInt(row))
}

// RowModuloFn creates a GenValueFn that returns the row number modulo a given
// value as a DInt
func RowModuloFn(modulo int) GenValueFn {
	return func(row int) parser.Datum {
		return parser.NewDInt(parser.DInt(row % modulo))
	}
}

// IntToEnglish returns an English (pilot style) string for the given integer,
// for example:
//   IntToEnglish(135) = "one-three-five"
func IntToEnglish(val int) string {
	if val < 0 {
		panic(val)
	}
	d := []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}

	var digits []string
	digits = append(digits, d[val%10])
	for val > 9 {
		val /= 10
		digits = append(digits, d[val%10])
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return strings.Join(digits, "-")
}

// RowEnglishFn is a GenValueFn which returns an English representation of the
// row number, as a DString
func RowEnglishFn(row int) parser.Datum {
	return parser.NewDString(IntToEnglish(row))
}

// ToRowFn creates a GenRowFn that returns rows of values generated by the given
// GenValueFns (one per column).
func ToRowFn(fn ...GenValueFn) GenRowFn {
	return func(row int) []parser.Datum {
		res := make([]parser.Datum, 0, len(fn))
		for _, f := range fn {
			res = append(res, f(row))
		}
		return res
	}
}
