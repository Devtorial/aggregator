package main

import (
	"archive/zip"
	"bufio"
	"encoding/xml"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/garyburd/redigo/redis"
	"github.com/robarchibald/configReader"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type aggregator struct {
	RedisServer         string
	RedisPort           int
	RedisPassword       string
	RedisMaxIdle        int
	RedisMaxConnections int
	ConcurrentDownloads int
}

type document struct {
	Path            string
	XML             string
	XMLName         xml.Name `xml:"document"`
	Type            string   `xml:"type"`
	Forum           string   `xml:"forum"`
	ForumTitle      string   `xml:"forum_title"`
	DiscussionTitle string   `xml:"discussion_title"`
	Language        string   `xml:"language"`
	GMTOffset       string   `xml:"gmt_offset"`
	TopicURL        string   `xml:"topic_url"`
	TopicText       string   `xml:"topic_text"`
	SpamScore       string   `xml:"spam_score"`
	PostNum         string   `xml:"post_num"`
	PostID          string   `xml:"post_id"`
	PostURL         string   `xml:"post_url"`
	PostDate        string   `xml:"post_date"`
	PostTime        string   `xml:"post_time"`
	Username        string   `xml:"username"`
	Post            string   `xml:"post"`
	Signature       string   `xml:"signature"`
	ExternalLinks   string   `xml:"external_links"`
	Country         string   `xml:"country"`
	MainImage       string   `xml:"main_image"`
}

var pool *redis.Pool
var downloadBusy chan bool

func main() {
	var config aggregator
	configReader.ReadFile("aggregator.conf", &config)

	if err := aggregate(readURL(), config.ConcurrentDownloads, newPool(config.RedisServer, config.RedisPort, config.RedisPassword, config.RedisMaxIdle, config.RedisMaxConnections)); err != nil {
		log.Fatal(err)
	}
}

func aggregate(url string, concurrentDownloads int, redisPool *redis.Pool) error {
	pool = redisPool
	defer pool.Close()

	downloadBusy = make(chan bool, concurrentDownloads)
	links, err := getLinks(url)
	if err != nil {
		return err
	}
	err = getFiles(links, "downloads")
	if err != nil {
		return err
	}
	return nil
}

func newPool(server string, port int, password string, maxIdle, maxConnections int) *redis.Pool {
	return &redis.Pool{
		MaxIdle:   maxIdle,
		MaxActive: maxConnections,
		Dial: func() (redis.Conn, error) {
			if password != "" {
				return redis.Dial("tcp", fmt.Sprintf("%s:%d", server, port), redis.DialPassword(password))
			}
			return redis.Dial("tcp", fmt.Sprintf("%s:%d", server, port))
		},
	}
}

func readURL() string {
	fmt.Print("Enter the URL to scan (default - http://feed.omgili.com/5Rh5AMTrc4Pv/mainstream/posts/):")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	if scanner.Text() == "" {
		return "http://feed.omgili.com/5Rh5AMTrc4Pv/mainstream/posts/"
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

	downloadBusy <- true
	if err := download(url, filename); err != nil {
		return err
	}
	<-downloadBusy
	unzipped, err := unzip(filename, outDir)
	if err != nil {
		return err
	}
	if err := saveToRedis(unzipped); err != nil {
		return err
	}
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
		if filepath.Ext(file) != ".xml" {
			return fmt.Errorf("invalid file type")
		}
		doc, err := parseData(file)
		if err != nil {
			return err
		}
		err = writeToRedis(doc)
		if err != nil {
			return err
		}
	}
	return nil
}

// Parse data to get key data, and to enable saving to JSON
// or other formats if desired
func parseData(path string) (*document, error) {
	doc := document{Path: path}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc.XML = string(data)
	err = xml.Unmarshal(data, &doc)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func writeToRedis(data *document) error {
	c := pool.Get()
	defer c.Close()

	// requirement is to be idempotent, so if post changes, we need
	// to delete the old value from the list. Use key store to keep
	// track of whether post is added already. This is an expensive
	// way to do this (2x storage). Look into better way
	v, err := redis.String(c.Do("GET", data.PostURL))
	if err != nil && err != redis.ErrNil {
		return err
	}

	// value is different, so update store
	if v != data.XML {
		if _, err := c.Do("SET", data.PostURL, data.XML); err != nil {
			return err
		}
		if v != "" { // only remove if it already exists
			if _, err := c.Do("LREM", "NEWS_XML", -1, v); err != nil { // remove old data
				return err
			}
		}
		if _, err := c.Do("RPUSH", "NEWS_XML", data.XML); err != nil {
			return err
		}
	}
	return nil
}
