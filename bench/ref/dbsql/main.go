package main

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
)

type testDriver struct{}
type testConn struct{}
type testStmt struct{ query string }
type testRows struct{ sent bool }
type testResult struct{}
type testTx struct{}

func (testDriver) Open(string) (driver.Conn, error)         { return testConn{}, nil }
func (testConn) Prepare(query string) (driver.Stmt, error)  { return testStmt{query}, nil }
func (testConn) Close() error                               { return nil }
func (testConn) Begin() (driver.Tx, error)                  { return testTx{}, nil }
func (s testStmt) Close() error                             { return nil }
func (s testStmt) NumInput() int                            { return 0 }
func (testStmt) Exec([]driver.Value) (driver.Result, error) { return testResult{}, nil }
func (testStmt) Query([]driver.Value) (driver.Rows, error)  { return &testRows{}, nil }
func (*testRows) Columns() []string                         { return []string{"id", "name"} }
func (*testRows) Close() error                              { return nil }
func (r *testRows) Next(dest []driver.Value) error {
	if r.sent {
		return io.EOF
	}
	r.sent = true
	dest[0], dest[1] = int64(4), "Ada"
	return nil
}
func (testResult) LastInsertId() (int64, error) { return 17, nil }
func (testResult) RowsAffected() (int64, error) { return 3, nil }
func (testTx) Commit() error                    { return nil }
func (testTx) Rollback() error                  { return nil }

func main() {
	sql.Register("fake", testDriver{})
	func() {
		defer func() { fmt.Println(recover()) }()
		sql.Register("fake", testDriver{})
	}()
	db, err := sql.Open("fake", "")
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	rows, err := db.Query("SELECT id, name FROM users")
	if err != nil {
		panic(err)
	}
	cols, _ := rows.Columns()
	fmt.Println("[" + cols[0] + " " + cols[1] + "]")
	fmt.Println(rows.Next())
	var id int64
	var name string
	if err := rows.Scan(&id, &name); err != nil {
		panic(err)
	}
	fmt.Printf("[Int(%d) Text(%s)]\n", id, name)
	fmt.Println(rows.Next())
	rows.Close()
	result, err := db.Exec("UPDATE users SET name = 'Ada'")
	if err != nil {
		panic(err)
	}
	affected, _ := result.RowsAffected()
	lastID, _ := result.LastInsertId()
	fmt.Println(affected)
	fmt.Println(lastID)
	var rowID int64
	var rowName string
	if err := db.QueryRow("SELECT id, name FROM users").Scan(&rowID, &rowName); err != nil {
		panic(err)
	}
	fmt.Printf("[Int(%d) Text(%s)]\n", rowID, rowName)
	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}
	txRows, err := tx.Query("SELECT id, name FROM users")
	if err != nil {
		panic(err)
	}
	txCols, _ := txRows.Columns()
	fmt.Println("[" + txCols[0] + " " + txCols[1] + "]")
	txCopy := tx
	if err := tx.Commit(); err != nil {
		panic(err)
	}
	fmt.Println("tx committed")
	_, err = txCopy.Query("SELECT id, name FROM users")
	fmt.Println(err)
	stmt, err := db.Prepare("SELECT 1")
	if err != nil {
		panic(err)
	}
	db.Close()
	_, err = db.Query("SELECT 1")
	fmt.Println(err)
	_, err = stmt.Query()
	fmt.Println(err)
	_ = stmt.Close()
	db2, err := sql.Open("fake", "")
	if err != nil {
		panic(err)
	}
	stmt2, err := db2.Prepare("SELECT 1")
	if err != nil {
		panic(err)
	}
	stmt2Copy := stmt2
	if err := stmt2.Close(); err != nil {
		panic(err)
	}
	_, err = stmt2Copy.Query()
	fmt.Println(err)
	_ = db2.Close()
}
