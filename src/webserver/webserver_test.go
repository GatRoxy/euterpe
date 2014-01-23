package webserver

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ironsmile/httpms/src/library"
)

const (
	TestPort = 9092
	TestRoot = "http_root"
)

func testUrl() string {
	return fmt.Sprintf("http://127.0.0.1:%d/", TestPort)
}

func testErrorAfter(seconds time.Duration, message string) chan int {
	ch := make(chan int)

	go func() {
		select {
		case _ = <-ch:
			close(ch)
			return
		case <-time.After(seconds * time.Second):
			close(ch)
			println(message)
			os.Exit(1)
		}
	}()

	return ch
}

func setUpServer() *Server {
	projRoot, err := getProjectRoot()

	if err != nil {
		println(err.Error())
		os.Exit(1)
	}

	var wsCfg ServerConfig
	wsCfg.Address = fmt.Sprintf(":%d", TestPort)
	wsCfg.Root = filepath.Join(projRoot, "test_files", TestRoot)

	return NewServer(wsCfg, nil)
}

func tearDownServer(srv *Server) {
	srv.Stop()
	ch := testErrorAfter(2, "Web server did not stop in time")
	srv.Wait()
	ch <- 42

	proto := "http"
	if srv.cfg.SSL {
		proto = "https"
	}
	url := fmt.Sprintf("%s://127.0.0.1:%d", proto, TestPort)

	_, err := http.Get(url)
	_, err = http.Get(url)

	if err == nil {
		println("Web server did not stop")
		os.Exit(1)
	}
}

func getProjectRoot() (string, error) {
	path, err := filepath.Abs(filepath.FromSlash("../.."))
	if err != nil {
		return "", err
	}
	return path, nil
}

func TestStaticFilesServing(t *testing.T) {
	srv := setUpServer()
	srv.Serve()
	defer tearDownServer(srv)

	testStaticFile := func(url, expected string) {

		resp, err := http.Get(url)

		if err != nil {
			t.Errorf(err.Error())
		}

		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("Unexpected response status code: %d", resp.StatusCode)
		}

		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			t.Errorf(err.Error())
		}

		if string(body) != expected {
			t.Errorf("Wrong static file found: %s", string(body))
		}
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/static", TestPort)
	testStaticFile(url, "This is a static file")

	url = fmt.Sprintf("http://127.0.0.1:%d/second/static", TestPort)
	testStaticFile(url, "Second static file")
}

func TestStartAndStop(t *testing.T) {

	_, err := http.Get(testUrl())

	if err == nil {
		t.Fatalf("Something is running on testing port %d", TestPort)
	}

	srv := setUpServer()
	srv.Serve()

	_, err = http.Get(testUrl())

	if err != nil {
		t.Errorf("Web server is not running %d", TestPort)
	}

	srv.Stop()

	ch := testErrorAfter(2, "Web server did not stop in time")
	srv.Wait()
	ch <- 42

	_, err = http.Get(testUrl())

	if err == nil {
		t.Errorf("The webserver was not stopped")
	}
}

func TestSSL(t *testing.T) {

	projectRoot, err := getProjectRoot()
	if err != nil {
		t.Fatalf("Could not determine project path: %s", err.Error())
	}
	certDir := filepath.Join(projectRoot, "test_files", "ssl")

	var wsCfg ServerConfig
	wsCfg.Address = fmt.Sprintf(":%d", TestPort)
	wsCfg.Root = TestRoot
	wsCfg.SSL = true
	wsCfg.SSLCert = filepath.Join(certDir, "cert.pem")
	wsCfg.SSLKey = filepath.Join(certDir, "key.pem")

	srv := NewServer(wsCfg, nil)
	srv.Serve()

	defer tearDownServer(srv)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	_, err = client.Get(fmt.Sprintf("https://127.0.0.1:%d", TestPort))

	if err != nil {
		t.Errorf("Error GETing a SSL url: %s", err.Error())
	}
}

func TestUserAuthentication(t *testing.T) {
	url := fmt.Sprintf("http://127.0.0.1:%d/static", TestPort)

	projRoot, err := getProjectRoot()

	if err != nil {
		t.Errorf(err.Error())
	}

	var wsCfg ServerConfig
	wsCfg.Address = fmt.Sprintf(":%d", TestPort)
	wsCfg.Root = filepath.Join(projRoot, "test_files", TestRoot)
	wsCfg.Auth = true
	wsCfg.AuthUser = "testuser"
	wsCfg.AuthPass = "testpass"

	srv := NewServer(wsCfg, nil)
	srv.Serve()
	defer tearDownServer(srv)

	resp, err := http.Get(url)

	if err != nil {
		t.Errorf(err.Error())
	}

	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Errorf("Expected 401 but got: %d", resp.StatusCode)
	}

	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth("testuser", "testpass")
	resp, err = client.Do(req)

	if err != nil {
		t.Errorf(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected 200 but got: %d", resp.StatusCode)
	}

	req, _ = http.NewRequest("GET", url, nil)
	req.SetBasicAuth("wronguser", "wrongpass")
	resp, err = client.Do(req)

	if err != nil {
		t.Errorf(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Errorf("Expected 401 but got: %d", resp.StatusCode)
	}
}

func TestSearchUrl(t *testing.T) {
	projRoot, _ := getProjectRoot()

	lib, _ := library.NewLocalLibrary("/tmp/test-web-search.db")
	err := lib.Initialize()

	if err != nil {
		t.Error(err)
	}

	defer lib.Truncate()

	lib.AddLibraryPath(filepath.Join(projRoot, "test_files", "library"))
	lib.Scan()

	ch := testErrorAfter(5, "Library in TestSearchUrl did not finish scaning on time")
	lib.WaitScan()
	ch <- 42

	var wsCfg ServerConfig
	wsCfg.Address = fmt.Sprintf(":%d", TestPort)
	wsCfg.Root = filepath.Join(projRoot, "test_files", TestRoot)

	srv := NewServer(wsCfg, lib)
	srv.Serve()
	defer tearDownServer(srv)

	/*
		The expected
		[
			{title:"", album:"", artist:"", track:0, id:0},
			...
			{title:"", album:"", artist:"", track:0, id:0}
		]
	*/

	url := fmt.Sprintf("http://127.0.0.1:%d/search/Album+Of+Tests", TestPort)
	resp, err := http.Get(url)

	if err != nil {
		t.Fatal(err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Unexpected response status code: %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")

	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Wrong content-type: %s", contentType)
	}

	responseBody, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		t.Error(err)
	}

	var results []library.SearchResult

	err = json.Unmarshal(responseBody, &results)

	if err != nil {
		t.Error(err)
	}

	if len(results) != 2 {
		t.Errorf("Expected two results from search but they were %d", len(results))
	}

	for _, result := range results {
		if result.Album != "Album Of Tests" {
			t.Errorf("Wrong album in search results: %s", result.Album)
		}
	}

	url = fmt.Sprintf("http://127.0.0.1:%d/search/Not+There", TestPort)
	resp, err = http.Get(url)

	if err != nil {
		t.Fatal(err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Unexpected response status code: %d", resp.StatusCode)
	}

	responseBody, err = ioutil.ReadAll(resp.Body)

	var noResults []library.SearchResult

	err = json.Unmarshal(responseBody, &noResults)

	if err != nil {
		t.Error(err)
	}

	if len(noResults) != 0 {
		t.Errorf("Expected no results from search but they were %d", len(noResults))
	}
}

func TestGetFileUrl(t *testing.T) {
	projRoot, _ := getProjectRoot()

	lib, err := library.NewLocalLibrary("/tmp/test-web-file-get.db")

	if err != nil {
		t.Fatal(err)
	}

	err = lib.Initialize()

	if err != nil {
		t.Error(err)
	}

	defer lib.Truncate()

	lib.AddLibraryPath(filepath.Join(projRoot, "test_files", "library"))
	lib.Scan()

	ch := testErrorAfter(5, "Library in TestGetFileUrl did not finish scaning on time")
	lib.WaitScan()
	ch <- 42

	var wsCfg ServerConfig
	wsCfg.Address = fmt.Sprintf(":%d", TestPort)
	wsCfg.Root = filepath.Join(projRoot, "test_files", TestRoot)

	srv := NewServer(wsCfg, lib)
	srv.Serve()
	defer tearDownServer(srv)

	found := lib.Search("Buggy Bugoff")

	if len(found) != 1 {
		t.Fatalf("Problem finding Buggy Bugoff test track")
	}

	trackID := found[0].ID

	url := fmt.Sprintf("http://127.0.0.1:%d/file/%d", TestPort, trackID)

	resp, err := http.Get(url)

	if err != nil {
		t.Fatal(err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Unexpected response status code: %d", resp.StatusCode)
	}

	responseBody, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		t.Fatal(err)
	}

	if len(responseBody) != 17314 {
		t.Errorf("Track size was not as expected. It was %d", len(responseBody))
	}

	contentLenHeader := resp.Header.Get("Content-Length")
	contentLenght, err := strconv.Atoi(contentLenHeader)

	if err != nil {
		t.Errorf("Content-Length was not integer. It was %s", contentLenHeader)
	}

	if contentLenght != 17314 {
		t.Errorf("Content-Length was not correct. It was %d", contentLenght)
	}
}

func TestGzipEncoding(t *testing.T) {
	//!TODO:
	/*
		On and off for
			* files
			* /search/
			* /file/
	*/
}

func TestRanges(t *testing.T) {
	//!TODO
}
