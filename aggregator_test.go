package main

import (
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestReadURL(t *testing.T) {
	// nothing received from stdin since it defaults to /dev/null
	url := readURL()
	if url != "http://bitly.com/nuvi-plz" {
		t.Error("expected default")
	}

	// create a fake stdin
	file, _ := ioutil.TempFile(os.TempDir(), "stdin")
	file.WriteString("http://www.google.com")
	stdin, _ := os.Open(file.Name())
	os.Stdin = stdin

	url = readURL()
	if url != "http://www.google.com" {
		t.Error("expected google", url)
	}
	os.Remove(file.Name())
	file.Close()
}

func TestGetLinks(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	// success
	data, err := getLinks("http://bitly.com/nuvi-plz")
	if err != nil || len(data) == 0 {
		t.Error("expected success on valid url", data)
	}

	// bad url
	_, err = getLinks("bogus")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetFiles(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	//create folder since it doesn't exist
	err := getFiles([]string{"http://bogusurl/small.zip"}, "testData/newFolder")
	if err == nil {
		t.Error("expected error")
	}
	os.RemoveAll("testData/newFolder")

	// all are local files, but big archives
	getFiles([]string{"http://bogus/1.zip", "http://bogus/2.zip", "http://bogus/3.zip", "http://bogus/4.zip", "http://bogus/5.zip", "http://bogus/6.zip", "http://bogus/7.zip", "http://bogus/8.zip", "http://bogus/9.zip", "http://bogus/10.zip"}, "testData/getfiles")
	time.Sleep(1 * time.Second)
	var count int
	for i := 1; i <= 10; i++ {
		files, _ := ioutil.ReadDir("testData/getfiles/" + strconv.Itoa(i))
		count += len(files)
	}
	if count != 2269 {
		t.Error("Expected to unzip ", count)
	}

	for i := 1; i <= 10; i++ {
		os.RemoveAll("testData/getfiles/" + strconv.Itoa(i))
	}
}

func TestDownloadUnzipSave(t *testing.T) {
	// success
	if err := downloadUnzipSave("testData/downloadUnzip", "http://www.colorado.edu/conflict/peace/download/peace_essay.ZIP"); err != nil {
		t.Error("expected success", err)
	}

	// downloaded, but can't save due to bad save folder
	if err := downloadUnzipSave("testData/!@#$%^&*()_+-=", "http://www.colorado.edu/conflict/peace/download/peace_essay.ZIP"); err == nil {
		t.Error("expected error due to invalid save folder")
	}

	// invalid zip
	if err := downloadUnzipSave("testData/downloadUnzip", "http://www.google.com/google.zip"); err == nil {
		t.Error("expected error on unzip")
	}
	os.RemoveAll("testData/downloadUnzip")
}

func TestGetDir(t *testing.T) {
	if dir := getDir("test.zip"); dir != "test" {
		t.Error("expected .zip to be removed", dir)
	}

	if dir := getDir("test.html"); dir != "" {
		t.Error("expected no directory to be returned since it isn't a zip", dir)
	}
}

func TestDownload(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	// success
	download("http://www.google.com", "testData/download/index.html") // download
	download("http://www.google.com", "testData/download/index.html") // exists
	if _, err := os.Stat("testData/download/index.html"); os.IsNotExist(err) {
		t.Error("expected download to succeed")
	}

	// bogus folder
	err := download("http://www.google.com", "testData/down?load/!@#$%?$%^&()+_=-")
	if err == nil {
		t.Error("expected invalid filename error")
	}

	// bogus filename
	err = download("http://www.google.com", "testData/download/!@#$%?$%^&()+_=-")
	if err == nil {
		t.Error("expected invalid filename error")
	}

	// bogus URL
	err = download("bogusurl", "testData/download/bogusurl.html")
	if err == nil {
		t.Error("expected fail due to bogus url")
	}
	os.RemoveAll("testData/download")
}

func TestUnzip(t *testing.T) {
	// invalid output path
	results, err := unzip("testdata/unzip/small.zip", "testData/!@#$%^&*()-_=+/\\")
	if err == nil {
		t.Error("expected error", err)
	}

	// success
	results, err = unzip("testData/unzip/small.zip", "testData/unzip/small")
	if err != nil || len(results) != 4 {
		t.Error("expected success", err, len(results))
	}

	// already unzipped
	results, err = unzip("testData/unzip/small.zip", "testData/unzip/small")
	if err != nil {
		t.Error("expected no error. already unzipped")
	}
	os.RemoveAll("testData/unzip/small")
}
