package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

type FileInfo struct {
	ID     string
	Name   string
	SHA256 []byte
}

type Storage struct {
	tempLoc        string
	filesByHashLoc string
	db             *sql.DB
	dbLock         sync.RWMutex // sqlite doesn't work well with concurrent writes using default settings, we need to avoid races on writing files for the same hash anyway so using a lock is the easiest solution
}

func StorageNew(storageLoc string) (*Storage, error) {
	s := &Storage{}

	mkdir := func(suffix string) (string, error) {
		loc := join(storageLoc, suffix)
		return loc, os.MkdirAll(loc, 0777)
	}

	var err error

	s.tempLoc, err = mkdir("temp")
	if err != nil {
		return nil, err
	}

	s.filesByHashLoc, err = mkdir("files")
	if err != nil {
		return nil, err
	}

	dbLoc := join(storageLoc, "db.sqlite")
	dbExists, err := fsExists(dbLoc)
	if err != nil {
		return nil, err
	}
	if !dbExists {
		db, err := sql.Open("sqlite3", dbLoc)
		s.db = db
		if err != nil {
			return nil, err
		}

		// name is unique index, because the task required retrieving uploaded file by name, and deleting by name. Generally in production better to use id instead.
		_, err = db.Exec(`CREATE TABLE files (
			id blob PRIMARY KEY,
			name text UNIQUE, 
			sha256 blob
		)`)
		if err != nil {
			return nil, err
		}

		_, err = db.Exec(`CREATE INDEX index_files_sha256 ON files (sha256)`)
		if err != nil {
			return nil, err
		}
	} else {
		db, err := sql.Open("sqlite3", dbLoc)
		s.db = db
		if err != nil {
			return nil, err
		}
	}

	return s, nil
}

func fsExists(loc string) (bool, error) {
	_, err := os.Stat(loc)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func randomFileID() string {
	return hex.EncodeToString(randomIDWithLen(32))
}

func randomTempFileName() string {
	return hex.EncodeToString(randomIDWithLen(32))
}

func randomIDWithLen(c int) []byte {
	res := make([]byte, c)
	_, err := io.ReadFull(rand.Reader, res)
	if err != nil {
		panic(err)
	}
	return res
}

func join(elem ...string) string {
	return filepath.Join(elem...)
}

func (s *Storage) saveInTempLocation(content io.Reader) (loc string, sha256Hash []byte, _ error) {
	id := randomTempFileName()
	loc = join(s.tempLoc, id)
	file, err := os.Create(loc)
	if err != nil {
		return "", nil, err
	}
	h := sha256.New()
	wr := io.MultiWriter(file, h)
	_, err = io.Copy(wr, content)
	if err != nil {
		os.Remove(loc)
		return "", nil, err
	}
	return loc, h.Sum(nil), nil
}

var ErrDuplicateName = errors.New("duplicate name used when saving file")

func (s *Storage) Save(ctx context.Context, name string, content io.ReadCloser) (id string, rerr error) {
	defer func() {
		err := content.Close()
		if err != nil {
			rerr = err
		}
	}()

	loc, sha256Hash, err := s.saveInTempLocation(content)
	if err != nil {
		return "", err
	}
	defer os.Remove(loc)

	s.dbLock.Lock()
	defer s.dbLock.Unlock()

	retErr := func(err error) {
		rerr = err
	}

	row := s.db.QueryRow("SELECT count(*) as count FROM files WHERE sha256=?", sha256Hash)
	var count int
	err = row.Scan(&count)
	if err != nil {
		retErr(err)
		return
	}

	// if the file already exists with the same hash do not copy it

	if count == 0 { // no file with the same hash yet
		err := os.Rename(loc, join(s.filesByHashLoc, hexEncode(sha256Hash)))
		if err != nil {
			retErr(err)
			return
		}
	}

	row = s.db.QueryRow("SELECT count(*) as count FROM files WHERE name=?", name)
	err = row.Scan(&count)
	if err != nil {
		retErr(err)
		return
	}

	if count != 0 {
		retErr(ErrDuplicateName)
		return
	}

	info := FileInfo{}
	info.ID = randomFileID()
	info.Name = name
	info.SHA256 = sha256Hash

	_, err = s.db.Exec("INSERT INTO files (id, name, sha256) VALUES (?,?,?)", info.ID, info.Name, info.SHA256)

	if err != nil {
		retErr(err)
		return
	}

	return info.ID, nil
}

func hexEncode(data []byte) string {
	return hex.EncodeToString(data)
}

var ErrNotFound = errors.New("requested record was not found")

func (s *Storage) getInfoSQL(ctx context.Context, sqlWhere string, params ...interface{}) (info FileInfo, _ error) {
	s.dbLock.RLock()
	defer s.dbLock.RUnlock()

	row := s.db.QueryRow("SELECT id, name, sha256 FROM files WHERE "+sqlWhere, params...)
	res := FileInfo{}
	err := row.Scan(&res.ID, &res.Name, &res.SHA256)
	if err == sql.ErrNoRows {
		return res, ErrNotFound
	}
	if err != nil {
		return res, err
	}
	return res, nil

}

func (s *Storage) GetInfo(ctx context.Context, id []byte) (info FileInfo, _ error) {
	return s.getInfoSQL(ctx, "id=?", id)
}

func (s *Storage) GetInfoByName(ctx context.Context, name string) (info FileInfo, _ error) {
	return s.getInfoSQL(ctx, "name=?", name)
}

func (s *Storage) GetContents(ctx context.Context, sha256Hash []byte) (io.ReadCloser, error) {
	return os.Open(join(s.filesByHashLoc, hexEncode(sha256Hash)))
}

func (s *Storage) Delete(ctx context.Context, info FileInfo) (rerr error) {
	s.dbLock.RLock()
	defer s.dbLock.RUnlock()

	retErr := func(err error) {
		rerr = err
	}

	row := s.db.QueryRow("SELECT count(*) as count FROM files WHERE sha256=?", info.SHA256)
	var count int
	err := row.Scan(&count)
	if err != nil {
		retErr(err)
		return
	}

	if count == 0 {
		return errors.New("no files with provides hash (must not happen)")
	}

	_, err = s.db.Exec("DELETE FROM files WHERE id = ?", info.ID)
	if err != nil {
		retErr(err)
		return
	}

	if count > 1 { // not the last file not need to delete
		return nil
	}

	// was last file with this hash

	if count == 0 {
		panic("count == 0 must return above")
	}

	err = os.Remove(join(s.filesByHashLoc, hexEncode(info.SHA256)))
	if err != nil {
		return errors.New("could not remove file with hash: " + hexEncode(info.SHA256))
	}

	return nil
}
