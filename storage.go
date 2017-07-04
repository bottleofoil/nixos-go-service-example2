package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"

	"bytes"

	_ "github.com/mattn/go-sqlite3"
)

type FileInfo struct {
	ID     string
	Name   string
	SHA256 []byte
}

type Storage struct {
}

func StorageNew(storageLoc string) (*Storage, error) {
	s := &Storage{}
	return s, nil
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

var ErrDuplicateName = errors.New("duplicate name used when saving file")
var ErrNotFound = errors.New("requested record was not found")

func (s *Storage) Save(ctx context.Context, name string, content io.ReadCloser) (id string, rerr error) {
	id = randomFileID()
	return id, nil
}

func hexEncode(data []byte) string {
	return hex.EncodeToString(data)
}

func (s *Storage) GetInfo(ctx context.Context, id []byte) (info FileInfo, _ error) {
	return FileInfo{}, nil
}

func (s *Storage) GetInfoByName(ctx context.Context, name string) (info FileInfo, _ error) {
	return FileInfo{}, nil
}

type readCloser struct {
	bytes.Reader
}

func (r *readCloser) Close() error {
	return nil
}

func (s *Storage) GetContents(ctx context.Context, sha256Hash []byte) (io.ReadCloser, error) {
	return &readCloser{}, nil
}

func (s *Storage) Delete(ctx context.Context, info FileInfo) (rerr error) {
	return nil
}
