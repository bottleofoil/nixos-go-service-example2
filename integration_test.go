package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"
)

const binName = "nixos-go-service-example2"

func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test, because -short flag was provided")
	}

	// make sure it's unique to avoid deleting some other dirs acidentally
	dataLoc := "nixos-go-service-example-db-xmdfus"
	err := os.RemoveAll(dataLoc)
	if err != nil {
		panic(err)
	}

	testHostName := "localhost:6063"

	cmdRun("go", "install", binName)

	// TODO: use WaitContext that will be available in go 1.9 to stop the server when done with the test (would allow multiple integration test cases), and no handcoded sleep needed
	cmd := cmdRunAsync(binName, "-host", testHostName, "-data-dir", dataLoc)
	time.Sleep(100 * time.Millisecond)
	defer cmd.Process.Kill()
	fmt.Println("started server")

	testHost := "http://" + testHostName
	testFile := []byte{0, 1, 2, 3}
	testName := "test-file.bin"

	// upload the file
	testR := bytes.NewReader(testFile)
	res, err := httpPut(testHost+"/"+testName, "", testR)
	if err != nil {
		t.Fatal("put failed: ", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal("invalid resp status code: ", res.StatusCode)
	}

	// download the file by name
	res, err = http.Get(testHost + "/" + testName)
	if err != nil {
		t.Fatal("get failed: ", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal("invalid resp status code: ", res.StatusCode)
	}
	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal("reading content failed")
	}
	if !reflect.DeepEqual(content, testFile) {
		t.Fatal("invalid resulting file content")
	}

	// delete the file
	res, err = httpDelete(testHost+"/"+testName, "", nil)
	if err != nil {
		t.Fatal("get failed: ", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal("invalid resp status code: ", res.StatusCode)
	}

	// downloading the deleted file should fail
	res, err = http.Get(testHost + "/" + testName)
	if err != nil {
		t.Fatal("get failed: ", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatal("expected 404 for deleted file, got: ", res.StatusCode)
	}

	// uploading the same file 10 times should only store it once
	// also test concurrency
	n := 100

	wg := sync.WaitGroup{}

	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			testR := bytes.NewReader(testFile)
			testName := "file" + strconv.Itoa(i)
			res, err := httpPut(testHost+"/"+testName, "", testR)
			if err != nil {
				t.Fatal("put failed: ", err)
			}
			defer res.Body.Close()
			if res.StatusCode != 200 {
				t.Fatal("invalid resp status code: ", res.StatusCode)
			}
		}()
	}

	wg.Wait()

	files, err := ioutil.ReadDir(filepath.Join(dataLoc, "files"))
	if err != nil {
		panic(err)
	}
	if len(files) != 1 {
		t.Fatal("files were stored invalid number of times when the content is the same:", len(files))
	}

	// retrieve the files (concurrently)
	wg = sync.WaitGroup{}

	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			testName := "file" + strconv.Itoa(i)
			res, err := http.Get(testHost + "/" + testName)
			if err != nil {
				t.Fatal("get failed: ", err)
			}
			defer res.Body.Close()
			if res.StatusCode != 200 {
				t.Fatal("invalid resp status code: ", res.StatusCode)
			}
			content, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Fatal("reading content failed, err: ", err)
			}
			if !reflect.DeepEqual(content, testFile) {
				t.Fatal("invalid resulting file content: ", content)
			}
		}()
	}

	wg.Wait()
}

func httpPut(url string, contentType string, body io.Reader) (resp *http.Response, err error) {
	req, err := http.NewRequest("PUT", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return http.DefaultClient.Do(req)
}

func httpDelete(url string, contentType string, body io.Reader) (resp *http.Response, err error) {
	req, err := http.NewRequest("DELETE", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return http.DefaultClient.Do(req)
}

func cmdRun(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
	return cmd
}

func cmdRunAsync(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	go func() {
		err := cmd.Run()
		if err != nil {
			panic(err)
		}
	}()
	return cmd
}
