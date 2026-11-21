package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Microsoft/go-winio"
	_ "modernc.org/sqlite"
	"net"
	"os"
	"strings"
	"time"
)

func must(e error) {
	if e != nil {
		panic(e)
	}
}

type request struct {
	Key  string
	ID   int
	Drop bool
}
type call struct {
	req  request
	conn net.Conn
}

func main() {
	p := os.Args[1]
	if !strings.Contains(p, "audit-6926") {
		panic("unscoped")
	}
	db, e := sql.Open("sqlite", p)
	must(e)
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, e = db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL; CREATE TABLE IF NOT EXISTS items(id INTEGER PRIMARY KEY,rev INTEGER,payload BLOB); CREATE TABLE IF NOT EXISTS receipts(k TEXT PRIMARY KEY,result INTEGER NOT NULL)")
	must(e)
	var sync int
	must(db.QueryRow("PRAGMA synchronous").Scan(&sync))
	if sync != 2 {
		panic(sync)
	}
	tx, e := db.Begin()
	must(e)
	_, e = tx.Exec("WITH RECURSIVE n(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM n WHERE i<10000) INSERT OR IGNORE INTO items SELECT i,0,zeroblob(320) FROM n")
	must(e)
	must(tx.Commit())
	l, e := winio.ListenPipe(os.Args[2], &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;OW)", InputBufferSize: 4096, OutputBufferSize: 4096})
	must(e)
	q := make(chan call, 32)
	go func() {
		for {
			c, e := l.Accept()
			must(e)
			go func() {
				var r request
				e := json.NewDecoder(c).Decode(&r)
				if e != nil {
					c.Close()
					return
				}
				q <- call{r, c}
			}()
		}
	}()
	fmt.Println("ready")
	for a := range q {
		group := []call{a}
		if os.Args[3] == "batch" {
			timer := time.NewTimer(2 * time.Millisecond)
		gather:
			for len(group) < 32 {
				select {
				case v := <-q:
					group = append(group, v)
				case <-timer.C:
					break gather
				}
			}
			timer.Stop()
		}
		tx, e := db.Begin()
		must(e)
		res := make([]int, len(group))
		for i, g := range group {
			r := g.req
			e = tx.QueryRow("SELECT result FROM receipts WHERE k=?", r.Key).Scan(&res[i])
			if e == sql.ErrNoRows {
				must(tx.QueryRow("UPDATE items SET rev=rev+1 WHERE id=? RETURNING rev", r.ID).Scan(&res[i]))
				_, e = tx.Exec("INSERT INTO receipts(k,result) VALUES(?,?)", r.Key, res[i])
			}
			must(e)
		}
		t := time.Now()
		must(tx.Commit())
		fmt.Fprintf(os.Stderr, "%.6f\n", float64(time.Since(t))/1e6)
		for i, g := range group {
			if !g.req.Drop {
				json.NewEncoder(g.conn).Encode(res[i])
			}
			g.conn.Close()
		}
	}
}
