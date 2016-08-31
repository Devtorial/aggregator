package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/robarchibald/configReader"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Aggregator struct {
}

type Document struct {
	Path            string
	Type            string
	Forum           string
	ForumTitle      string
	DiscussionTitle string
	Language        string
	GMTOffset       string
	TopicURL        string
	TopicText       string
	SpamScore       string
	PostNum         int
	PostID          string
	PostURL         string
	PostDate        string
	PostTime        string
	Username        string
	Post            string
	Signature       string
	ExternalLinks   string
	Country         string
	MainImage       string
}

func main() {
	aggregator := configReader.ReadFile("aggregator.conf", &Aggregator{})
	fmt.Print(aggregator)

	url := readURL()
	links, err := getLinks(url)
	if err != nil {
		log.Fatal(err)
	}
	getFiles(links, "downloads")
}

func readURL() string {
	fmt.Print("Enter the URL to scan (default - http://bitly.com/nuvi-plz):")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	if scanner.Text() == "" {
		return "http://bitly.com/nuvi-plz"
	}
	return scanner.Text()
}

func getLinks(url string) ([]string, error) {
	doc, err := goquery.NewDocument(url)
	if err != nil {
		return nil, err
	}

	links := []string{}
	doc.Find("table tr td a").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists && strings.HasSuffix(href, ".zip") {
			if !strings.HasPrefix(href, "http") {
				href = doc.Url.String() + href
			}
			links = append(links, href)
		}
	})
	return links, nil
}

func getFiles(links []string, downloadFolder string) error {
	if _, err := os.Stat(downloadFolder); os.IsNotExist(err) {
		os.MkdirAll(downloadFolder, 755)
	}
	errors := make(chan error, len(links)) // buffered so that it won't block
	var wg sync.WaitGroup
	for _, link := range links {
		wg.Add(1)
		go func(link string) {
			defer wg.Done()
			errors <- downloadUnzipSave(downloadFolder, link)
		}(link)
	}
	wg.Wait()
	close(errors)
	var outErr error
	for err := range errors {
		if err != nil {
			outErr = err
		}
	}
	return outErr
}

func downloadUnzipSave(downloadFolder, url string) error {
	filename := filepath.Join(downloadFolder, url[strings.LastIndex(url, "/"):])
	outDir := getDir(filename)

	if err := download(url, filename); err != nil {
		return err
	}
	unzipped, err := unzip(filename, outDir)
	if err != nil {
		return err
	}
	saveToRedis(unzipped)
	return nil
}

func getDir(filename string) string {
	if !strings.HasSuffix(strings.ToLower(filename), ".zip") {
		return ""
	}
	return filename[:len(filename)-4]
}

func download(url, filename string) error {
	if _, err := os.Stat(filename); err == nil || os.IsExist(err) {
		return nil // already downloaded
	}

	r, err := http.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	dir := filepath.Dir(filename)
	err = os.MkdirAll(dir, 755)
	if err != nil {
		return err
	}
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, r.Body)
	return err
}

func unzip(src, dest string) ([]string, error) {
	unzippedFiles := []string{}
	if _, err := os.Stat(dest); err == nil || os.IsExist(err) {
		return unzippedFiles, nil // already unzipped
	}
	r, err := zip.OpenReader(src)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	os.MkdirAll(dest, 0755)

	extractAndWriteFile := func(f *zip.File) error {
		unzipped, err := f.Open()
		if err != nil {
			return err
		}

		path := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}

			_, err = io.Copy(f, unzipped)
			if err != nil {
				return err
			}

			err = f.Close()
			if err != nil {
				return err
			}
			unzippedFiles = append(unzippedFiles, path)
		}
		unzipped.Close()
		return nil
	}

	for i := 0; i < len(r.File); i++ {
		err := extractAndWriteFile(r.File[i])
		if err != nil {
			return nil, err
		}
	}

	return unzippedFiles, nil
}

func saveToRedis(filename []string) error {
	for _, file := range filename {
		err := writeToRedis(parseData(file))
		if err != nil {
			return err
		}
	}
	return nil
}

func parseData(path string) *Document {
	return &Document{Path: path}
}

func writeToRedis(data *Document) error {
	//save
	return nil
}
