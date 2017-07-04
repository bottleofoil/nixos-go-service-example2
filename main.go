package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"strings"
)

type Response struct {
	Error string `json:"error"`
}

func main() {

	argHost := ""
	flag.StringVar(&argHost, "host", "localhost:80", "host name and port number for server to use")
	argDataDir := ""
	flag.StringVar(&argDataDir, "data-dir", "./files-db", "directory to save sqlite database and files")
	flag.Parse()

	storage, err := StorageNew(argDataDir)
	if err != nil {
		panic(err)
	}

	server := &http.Server{
		Addr: argHost,
	}

	log := Logger{}

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {

		logReq := func(msg string) {
			log.Info(req.Method + " " + req.URL.Path + " " + msg)
		}
		logReqError := func(msg string) {
			log.Error(req.Method + " " + req.URL.Path + " " + msg)
		}

		log.Info(req.Method + " " + req.URL.Path)

		respondJSON := func(obj interface{}) {
			b, err := json.Marshal(obj)
			if err != nil {
				// must not happen, because we always pass ordinary structs to json.Marshal
				logReqError("can't marshal response, err: " + err.Error())
				return
			}
			_, err = w.Write(b)
			logReq("json response: " + string(b))
			if err != nil {
				logReqError(err.Error())
			}
		}

		respondWithError := func(err error, code int) {
			if err == nil {
				panic("no error")
			}
			w.WriteHeader(code)
			logReqError(err.Error())
			respondJSON(Response{Error: err.Error()})
		}

		ctx := req.Context()

		if req.Method == "PUT" {
			handlePUT(ctx, storage, w, req, respondWithError)
			return
		}

		if req.Method == "GET" {
			handleGET(ctx, storage, w, req, respondWithError, logReqError)
			return
		}

		if req.Method == "DELETE" {
			handleDELETE(ctx, storage, w, req, respondWithError)
			return
		}

		logReq("invalid request method: " + req.Method)

	})

	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

func handlePUT(ctx context.Context, storage *Storage, w http.ResponseWriter, req *http.Request, respondWithError func(error, int)) {
	fileName := strings.TrimPrefix(req.URL.Path, "/")
	err := validateFileName(fileName)
	if err != nil {
		respondWithError(err, 400)
		return
	}
	_, err = storage.Save(ctx, fileName, req.Body)
	if err != nil {
		respondWithError(err, 500)
		return
	}
	return
}

func handleGET(ctx context.Context, storage *Storage, w http.ResponseWriter, req *http.Request, respondWithError func(error, int), logReqError func(string)) {
	fileName := strings.TrimPrefix(req.URL.Path, "/")
	err := validateFileName(fileName)
	if err != nil {
		w.WriteHeader(404)
		return
	}

	info, err := storage.GetInfoByName(ctx, fileName)
	if err == ErrNotFound {
		w.WriteHeader(404)
		return
	}
	if err != nil {
		respondWithError(err, 500)
		return
	}

	r, err := storage.GetContents(ctx, info.SHA256)
	if err == ErrNotFound {
		w.WriteHeader(404)
		return
	}
	if err != nil {
		respondWithError(err, 500)
		return
	}
	_, err = io.Copy(w, r)
	if err != nil {
		logReqError(err.Error())
		return
	}
	return
}

func handleDELETE(ctx context.Context, storage *Storage, w http.ResponseWriter, req *http.Request, respondWithError func(error, int)) {
	fileName := strings.TrimPrefix(req.URL.Path, "/")
	err := validateFileName(fileName)
	if err != nil {
		respondWithError(err, 400)
		return
	}

	info, err := storage.GetInfoByName(ctx, fileName)
	if err != nil {
		respondWithError(err, 500)
		return
	}

	err = storage.Delete(ctx, info)
	if err != nil {
		respondWithError(err, 500)
		return
	}

	return
}

func validateFileName(fileName string) error {
	// TODO: validate filename to show user friendly error messages, does not matter for security because we store it escaped in the db
	return nil
}
