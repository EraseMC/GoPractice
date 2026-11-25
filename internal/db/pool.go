package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"hash/crc32"
	"log/slog"
	"strings"
	"sync"

	"github.com/go-sql-driver/mysql"
)

type Stmt struct {
	SQL  string
	Args []any
}

type Query struct {
	Stmts []Stmt
	Tx    bool
}

func Make(sql string, args ...any) Query {
	return Query{Stmts: []Stmt{{SQL: sql, Args: args}}}
}

func Transaction(stmts ...Stmt) Query {
	return Query{Stmts: stmts, Tx: true}
}

type Row map[string]any

type Result struct {
	Rows     []Row
	InsertID int64
	Batch    []Result
	Err      error
}

type job struct {
	q    Query
	done chan Result
}

type worker struct {
	number  int
	mu      sync.Mutex
	cond    *sync.Cond
	queue   []job
	stopped bool
}

type Pool struct {
	db      *sql.DB
	log     *slog.Logger
	workers []*worker
	wg      sync.WaitGroup
	once    sync.Once
}

func NewPool(d *sql.DB, n int, log *slog.Logger) *Pool {
	p := &Pool{db: d, log: log}
	for i := range max(1, n) {
		w := &worker{number: i + 1}
		w.cond = sync.NewCond(&w.mu)
		p.workers = append(p.workers, w)
		p.wg.Add(1)
		go p.run(w)
	}
	return p
}

func (p *Pool) Execute(q Query, shard string) {
	p.dispatch(job{q: q}, shard)
}

func (p *Pool) Query(q Query, shard string) <-chan Result {
	done := make(chan Result, 1)
	p.dispatch(job{q: q, done: done}, shard)
	return done
}

func (p *Pool) dispatch(j job, shard string) {
	idx := 0
	if shard != "" {
		idx = int(crc32.ChecksumIEEE([]byte(shard)) % uint32(len(p.workers)))
	}
	w := p.workers[idx]
	w.mu.Lock()
	if w.stopped {
		w.mu.Unlock()
		if j.done != nil {
			j.done <- Result{Err: errors.New("MySQL pool is closed")}
		}
		return
	}
	w.queue = append(w.queue, j)
	w.mu.Unlock()
	w.cond.Signal()
}

func (p *Pool) Pending() int {
	n := 0
	for _, w := range p.workers {
		w.mu.Lock()
		n += len(w.queue)
		w.mu.Unlock()
	}
	return n
}

func (p *Pool) run(w *worker) {
	defer p.wg.Done()
	for {
		w.mu.Lock()
		for len(w.queue) == 0 && !w.stopped {
			w.cond.Wait()
		}
		if len(w.queue) == 0 && w.stopped {
			w.mu.Unlock()
			return
		}
		j := w.queue[0]
		w.queue[0] = job{}
		w.queue = w.queue[1:]
		w.mu.Unlock()

		res := p.process(w, j.q)
		if res.Err != nil {
			p.log.Error("MySQL: " + res.Err.Error())
		}
		if j.done != nil {
			j.done <- res
		}
	}
}

func (p *Pool) process(w *worker, q Query) Result {
	res := p.runQuery(q)
	if res.Err == nil || !connectionLost(res.Err) {
		return res
	}
	p.log.Warn(fmt.Sprintf("MySQL worker #%d lost connection, reconnecting...", w.number))
	return p.runQuery(q)
}

func (p *Pool) runQuery(q Query) Result {
	ctx := context.Background()
	if !q.Tx {
		if len(q.Stmts) == 0 {
			return Result{}
		}
		return execute(ctx, p.db, q.Stmts[0])
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Result{Err: err}
	}
	out := Result{Batch: make([]Result, 0, len(q.Stmts))}
	for _, s := range q.Stmts {
		r := execute(ctx, tx, s)
		if r.Err != nil {
			_ = tx.Rollback()
			return Result{Err: r.Err}
		}
		out.Batch = append(out.Batch, r)
	}
	if err := tx.Commit(); err != nil {
		return Result{Err: err}
	}
	return out
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func execute(ctx context.Context, e execer, s Stmt) Result {
	args := make([]any, len(s.Args))
	for i, a := range s.Args {
		if b, ok := a.(bool); ok {
			if b {
				a = 1
			} else {
				a = 0
			}
		}
		args[i] = a
	}
	if !isSelect(s.SQL) {
		r, err := e.ExecContext(ctx, s.SQL, args...)
		if err != nil {
			return Result{Err: err}
		}
		id, _ := r.LastInsertId()
		return Result{InsertID: id}
	}
	rows, err := e.QueryContext(ctx, s.SQL, args...)
	if err != nil {
		return Result{Err: err}
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return Result{Err: err}
	}
	var out []Row
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return Result{Err: err}
		}
		row := make(Row, len(cols))
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				row[c] = string(b)
			} else {
				row[c] = vals[i]
			}
		}
		out = append(out, row)
	}
	return Result{Rows: out, Err: rows.Err()}
}

func isSelect(sql string) bool {
	s := strings.TrimSpace(sql)
	return len(s) >= 6 && strings.EqualFold(s[:6], "SELECT")
}

func connectionLost(err error) bool {
	if errors.Is(err, driver.ErrBadConn) || errors.Is(err, mysql.ErrInvalidConn) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "server has gone away") ||
		strings.Contains(msg, "Lost connection") ||
		strings.Contains(msg, "Connection refused") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "broken pipe")
}

func (p *Pool) Close() {
	p.once.Do(func() {
		for _, w := range p.workers {
			w.mu.Lock()
			w.stopped = true
			w.mu.Unlock()
			w.cond.Broadcast()
		}
		p.wg.Wait()
	})
}
