package store

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"

	"github.com/Andrew-Sem/gc/framework/journal"
	"github.com/Andrew-Sem/gc/specs/contract"
	_ "modernc.org/sqlite"
)

var ErrUnreadable = errors.New("unreadable journal")

var sqliteHeader = []byte("SQLite format 3\x00")

var kinds = map[string]func([]byte) (contract.Fact, error){
	"RepositoryConnected":       decode[contract.RepositoryConnected],
	"RepositoryDisconnected":    decode[contract.RepositoryDisconnected],
	"PrCreated":                 decode[contract.PrCreated],
	"PrStateChanged":            decode[contract.PrStateChanged],
	"SweepRequested":            decode[contract.SweepRequested],
	"RunRequested":              decode[contract.RunRequested],
	"RepositorySettingsChanged": decode[contract.RepositorySettingsChanged],
	"AdmissionApproved":         decode[contract.AdmissionApproved],
	"AdmissionDeclined":         decode[contract.AdmissionDeclined],
	"MergeApproved":             decode[contract.MergeApproved],
	"MergeDeclined":             decode[contract.MergeDeclined],
	"MergeFailed":               decode[contract.MergeFailed],
	"AgentRunStarted":           decode[contract.AgentRunStarted],
	"AgentRunFinished":          decode[contract.AgentRunFinished],
}

func decode[F contract.Fact](data []byte) (contract.Fact, error) {
	var fact F
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fact, errors.New("the fact is null")
	}
	err := json.Unmarshal(data, &fact)
	return fact, err
}

type Journal struct {
	mu             sync.Mutex
	path           string
	file           os.FileInfo
	db             *sql.DB
	memory         journal.Journal[contract.Fact]
	onWriteFailure func(error)
}

func Open(path string, onWriteFailure func(error)) (*Journal, error) {
	if err := checkDatabase(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	j := &Journal{path: path, db: db, onWriteFailure: onWriteFailure}
	if err := j.load(); err != nil {
		db.Close()
		return nil, err
	}
	if j.file, err = os.Stat(path); err != nil {
		db.Close()
		return nil, err
	}
	return j, nil
}

func checkDatabase(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	header := make([]byte, len(sqliteHeader))
	n, err := io.ReadFull(file, header)
	if n == 0 && errors.Is(err, io.EOF) {
		return nil
	}
	if !bytes.Equal(header[:n], sqliteHeader) {
		return errors.New("the file is not a SQLite database")
	}
	return nil
}

func (j *Journal) load() error {
	for _, statement := range []string{
		"PRAGMA locking_mode=EXCLUSIVE",
		"BEGIN EXCLUSIVE",
		"CREATE TABLE IF NOT EXISTS facts (seq INTEGER PRIMARY KEY AUTOINCREMENT, kind TEXT NOT NULL, data TEXT NOT NULL)",
		"COMMIT",
	} {
		if _, err := j.db.Exec(statement); err != nil {
			return err
		}
	}
	rows, err := j.db.Query("SELECT kind, data FROM facts ORDER BY seq")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var data []byte
		if err := rows.Scan(&kind, &data); err != nil {
			return fmt.Errorf("%w: %v", ErrUnreadable, err)
		}
		decodeKind, known := kinds[kind]
		if !known {
			return fmt.Errorf("%w: unknown fact %q", ErrUnreadable, kind)
		}
		fact, err := decodeKind(data)
		if err != nil {
			return fmt.Errorf("%w: fact %q: %v", ErrUnreadable, kind, err)
		}
		j.memory.Append(fact)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	return nil
}

func (j *Journal) Append(facts ...contract.Fact) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if err := j.write(facts); err != nil {
		j.onWriteFailure(err)
		return
	}
	j.memory.Append(facts...)
}

func (j *Journal) write(facts []contract.Fact) error {
	current, err := os.Stat(j.path)
	if err != nil {
		return err
	}
	if !os.SameFile(current, j.file) {
		return errors.New("the journal file was replaced")
	}
	tx, err := j.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, fact := range facts {
		kind := reflect.TypeOf(fact).Name()
		if _, known := kinds[kind]; !known {
			return fmt.Errorf("unknown fact %q", kind)
		}
		data, err := json.Marshal(fact)
		if err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT INTO facts (kind, data) VALUES (?, ?)", kind, string(data)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (j *Journal) Facts() []contract.Fact {
	return j.memory.Facts()
}

func (j *Journal) Subscribe() (<-chan struct{}, func()) {
	return j.memory.Subscribe()
}

func (j *Journal) Close() error {
	return j.db.Close()
}
