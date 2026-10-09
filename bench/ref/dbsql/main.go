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
type testRows struct {
	columns []string
	rows    [][]driver.Value
	next    int
}
type testResult struct{}
type testTx struct{}
type pgDriver struct{}

func (testDriver) Open(string) (driver.Conn, error)         { return testConn{}, nil }
func (pgDriver) Open(string) (driver.Conn, error)           { return testConn{}, nil }
func (testConn) Prepare(query string) (driver.Stmt, error)  { return testStmt{query}, nil }
func (testConn) Close() error                               { return nil }
func (testConn) Begin() (driver.Tx, error)                  { return testTx{}, nil }
func (s testStmt) Close() error                             { return nil }
func (s testStmt) NumInput() int                            { return 0 }
func (testStmt) Exec([]driver.Value) (driver.Result, error) { return testResult{}, nil }
func (s testStmt) Query([]driver.Value) (driver.Rows, error) {
	columns, rows := table(s.query)
	return &testRows{columns: columns, rows: rows}, nil
}
func (r *testRows) Columns() []string { return r.columns }
func (r *testRows) Close() error      { return nil }
func (r *testRows) Next(dest []driver.Value) error {
	if r.next >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.next])
	r.next++
	return nil
}
func (testResult) LastInsertId() (int64, error) { return 17, nil }
func (testResult) RowsAffected() (int64, error) { return 3, nil }
func (testTx) Commit() error                    { return nil }
func (testTx) Rollback() error                  { return nil }

// table is the columns and rows the fake server answers a query text with.
func table(q string) ([]string, [][]driver.Value) {
	switch q {
	case "SELECT id, name FROM users":
		return []string{"id", "name"}, [][]driver.Value{{int64(4), "Ada"}}
	case "SELECT typed":
		return []string{"id", "score", "name", "ok", "raw"}, [][]driver.Value{{int64(4), 2.5, "Ada", int64(1), "raw"}}
	case "SELECT nulls":
		return []string{"a", "b", "c", "d", "e"}, [][]driver.Value{{nil, nil, nil, nil, nil}}
	case "SELECT text":
		return []string{"n", "f", "b", "t"}, [][]driver.Value{{"12", "1.5", "t", "abc"}}
	case "SELECT bad":
		return []string{"n"}, [][]driver.Value{{"abc"}}
	case "SELECT ints":
		return []string{"n"}, [][]driver.Value{{int64(7)}}
	case "SELECT float":
		return []string{"f"}, [][]driver.Value{{float64(3)}}
	case "SELECT fraction":
		return []string{"f"}, [][]driver.Value{{float64(1.5)}}
	case "SELECT bits":
		return []string{"a", "b"}, [][]driver.Value{{int64(1), int64(0)}}
	case "SELECT badbit":
		return []string{"a"}, [][]driver.Value{{int64(2)}}
	case "SELECT bytes":
		return []string{"b"}, [][]driver.Value{{"bytes"}}
	case "SELECT null int":
		return []string{"n"}, [][]driver.Value{{nil}}
	}
	return []string{}, [][]driver.Value{}
}

// tryPanic runs f and prints the panic value (Register's refusal is a panic in Go).
func tryPanic(f func()) {
	defer func() { fmt.Println(recover()) }()
	f()
}

// otherCore is what core 1 does in the strict test: the driver is visible there too.
func otherCore() {
	tryPanic(func() { sql.Register("fake", testDriver{}) })
	db, err := sql.Open("fake", "")
	if err != nil {
		panic(err)
	}
	var id int64
	var name string
	if err := db.QueryRow("SELECT id, name FROM users").Scan(&id, &name); err != nil {
		panic(err)
	}
	fmt.Println("core 1 scans", id, name)
	_, err = sql.Open("missing", "")
	fmt.Println(err)
}

func main() {
	sql.Register("fake", testDriver{})
	tryPanic(func() { sql.Register("fake", testDriver{}) })
	sql.Register("", testDriver{})
	sql.Register("pg", pgDriver{})
	fmt.Println(sql.Drivers())
	otherCore()
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
	var early int64
	fmt.Println(rows.Scan(&early))
	fmt.Println(rows.Next())
	var cid int64
	var cname string
	if err := rows.Scan(&cid, &cname); err != nil {
		panic(err)
	}
	fmt.Println(cid, cname)
	fmt.Printf("[Int(%d) Text(%s)]\n", cid, cname)
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
	fmt.Println(rowID, rowName)
	var wide int64
	fmt.Println(db.QueryRow("SELECT id, name FROM users").Scan(&wide))
	var typedID int64
	var typedScore float64
	var typedName string
	var typedOK bool
	var typedRaw []byte
	if err := db.QueryRow("SELECT typed").Scan(&typedID, &typedScore, &typedName, &typedOK, &typedRaw); err != nil {
		panic(err)
	}
	fmt.Println(typedID, typedScore, typedName, typedOK, string(typedRaw))
	var nInt sql.NullInt64
	var nFloat sql.NullFloat64
	var nString sql.NullString
	var nBool sql.NullBool
	var nBytes []byte
	if err := db.QueryRow("SELECT nulls").Scan(&nInt, &nFloat, &nString, &nBool, &nBytes); err != nil {
		panic(err)
	}
	fmt.Println(!nInt.Valid, !nFloat.Valid, !nString.Valid, !nBool.Valid, nBytes == nil)
	var nullInt int64
	fmt.Println(db.QueryRow("SELECT null int").Scan(&nullInt))
	var textN int64
	var textF float64
	var textB bool
	var textT string
	if err := db.QueryRow("SELECT text").Scan(&textN, &textF, &textB, &textT); err != nil {
		panic(err)
	}
	fmt.Println(textN, textF, textB, textT)
	var badInt int64
	fmt.Println(db.QueryRow("SELECT bad").Scan(&badInt))
	var ints string
	if err := db.QueryRow("SELECT ints").Scan(&ints); err != nil {
		panic(err)
	}
	fmt.Println(ints)
	var intBytes []byte
	if err := db.QueryRow("SELECT ints").Scan(&intBytes); err != nil {
		panic(err)
	}
	fmt.Println(string(intBytes))
	var fracBytes []byte
	if err := db.QueryRow("SELECT fraction").Scan(&fracBytes); err != nil {
		panic(err)
	}
	fmt.Println(string(fracBytes))
	var fraction int64
	fmt.Println(db.QueryRow("SELECT fraction").Scan(&fraction))
	var whole int64
	if err := db.QueryRow("SELECT float").Scan(&whole); err != nil {
		panic(err)
	}
	fmt.Println(whole)
	var bits, bitsOff bool
	if err := db.QueryRow("SELECT bits").Scan(&bits, &bitsOff); err != nil {
		panic(err)
	}
	fmt.Println(bits, bitsOff)
	var badBit bool
	fmt.Println(db.QueryRow("SELECT badbit").Scan(&badBit))
	var bytesCell []byte
	if err := db.QueryRow("SELECT bytes").Scan(&bytesCell); err != nil {
		panic(err)
	}
	fmt.Println(string(bytesCell))
	var noRow int64
	fmt.Println(db.QueryRow("SELECT none").Scan(&noRow))
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)
	fmt.Println("limits changed after use")
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
	tx, err := db2.Begin()
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
	_ = db2.Close()
}
