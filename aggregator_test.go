package main

import (
	"github.com/garyburd/redigo/redis"
	"github.com/rafaeljusto/redigomock"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestAggregate(t *testing.T) {
	// sucess even though there are no links at this URL
	err := aggregate("http://www.google.com", 5, newMockPool(nil))
	if err != nil {
		t.Error("expected success")
	}
}

func TestReadURL(t *testing.T) {
	// nothing received from stdin since it defaults to /dev/null
	url := readURL()
	if url != "http://feed.omgili.com/5Rh5AMTrc4Pv/mainstream/posts/" {
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
	data, err := getLinks("http://feed.omgili.com/5Rh5AMTrc4Pv/mainstream/posts/")
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

	pool = newMockPool(nil)
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
	downloadBusy = make(chan bool, 100)
	conn := redigomock.NewConn()
	pool = newMockPool(conn)

	// fail on redis (zip file is cached)
	if err := downloadUnzipSave("testData/unzip", "http://test.com/small.zip"); err == nil {
		t.Error("expected fail on redis")
	}

	// success
	conn.GenericCommand("GET").ExpectError(redis.ErrNil)
	conn.GenericCommand("SET").Expect("success")
	conn.GenericCommand("RPUSH").Expect("success")
	if err := downloadUnzipSave("testData/unzip", "http://test.com/small.zip"); err != nil {
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
	os.RemoveAll("testData/unzip/small") // beginning AND end since another test unzips too
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

func newMockPool(conn *redigomock.Conn) *redis.Pool {
	return &redis.Pool{
		MaxIdle:   80,
		MaxActive: 12000,
		Dial: func() (redis.Conn, error) {
			if conn != nil {
				return conn, nil
			}
			return redigomock.NewConn(), nil
		},
	}
}

func TestSaveToRedis(t *testing.T) {
	// success
	conn := redigomock.NewConn()
	pool = newMockPool(conn)
	xml, _ := ioutil.ReadFile("testData/xml/valid.xml")
	conn.Command("GET", "http://www.haberler.com/konkoglu-ndan-taziya-ziyareti-8731165-haberi/").Expect("different")
	conn.Command("SET", "http://www.haberler.com/konkoglu-ndan-taziya-ziyareti-8731165-haberi/", string(xml)).Expect("Success")
	conn.Command("LREM", "NEWS_XML", -1, "different").Expect("Success")
	conn.Command("RPUSH", "NEWS_XML", string(xml)).Expect("Success")
	err := saveToRedis([]string{"testData/xml/valid.xml"})
	if err != nil {
		t.Error("expected success", err)
	}

	// data matches
	conn = redigomock.NewConn()
	pool = newMockPool(conn)
	conn.GenericCommand("GET").Expect(string(xml))
	err = saveToRedis([]string{"testData/xml/valid.xml"})
	if err != nil {
		t.Error("expected success", err)
	}

	// error on get
	conn = redigomock.NewConn()
	pool = newMockPool(conn)
	conn.GenericCommand("GET").ExpectError(redis.ErrPoolExhausted)
	err = saveToRedis([]string{"testData/xml/valid.xml"})
	if err != redis.ErrPoolExhausted {
		t.Error("expected fail on get", err)
	}

	// error on set
	conn = redigomock.NewConn()
	pool = newMockPool(conn)
	conn.GenericCommand("GET").Expect("different")
	conn.GenericCommand("SET").ExpectError(redis.ErrNil)
	err = saveToRedis([]string{"testData/xml/valid.xml"})
	if err != redis.ErrNil {
		t.Error("expected fail on set", err)
	}

	// error on LREM
	conn = redigomock.NewConn()
	pool = newMockPool(conn)
	conn.GenericCommand("GET").Expect("different")
	conn.GenericCommand("SET").Expect("success")
	conn.GenericCommand("LREM").ExpectError(redis.ErrNil)
	err = saveToRedis([]string{"testData/xml/valid.xml"})
	if err != redis.ErrNil {
		t.Error("expected fail on lrem", err)
	}

	// error on RPUSH
	conn = redigomock.NewConn()
	pool = newMockPool(conn)
	conn.GenericCommand("GET").Expect("different")
	conn.GenericCommand("SET").Expect("success")
	conn.GenericCommand("LREM").Expect("success")
	conn.GenericCommand("RPUSH").ExpectError(redis.ErrNil)
	err = saveToRedis([]string{"testData/xml/valid.xml"})
	if err != redis.ErrNil {
		t.Error("expected fail on rpush", err)
	}

	// error
	err = saveToRedis([]string{"testData/xml/!@#$%^&*()_+?.xml"})
	if err == nil {
		t.Error("expected error")
	}
}

func TestParseData(t *testing.T) {
	topicText := `
Konkoğlu'ndan Taziya Ziyareti 26 Ağustos 2016 Cuma 16:23 Gaziantep Sanayi Odası (GSO) Yönetim Kurulu Başkanı Adil Konukoğlu, Gaziantep’te terör saldırısında hayatını kaybeden vatandaşların ailelerine taziye ziyaretinde bulunarak, başsağlığı diledi. 
Gaziantep Sanayi Odası (GSO) Yönetim Kurulu Başkanı Adil Konukoğlu , Gaziantep 'te terör saldırısında hayatını kaybeden vatandaşların ailelerine taziye ziyaretinde bulunarak, başsağlığı diledi.Geçmiş olsun ziyareti için Gaziantep 'e gelen Gümrük ve Ticaret Bakanı Bülent Tüfenkci ile Türkiye Odalar ve Borsalar Birliği ( TOBB ) Başkanı Rifat Hisarcıklıoğlu , geçtiğimiz hafta bir kına gecesinde meydana gelen terör saldırısının gerçekleştirildiği sokağı gezdi. Gaziantep Valisi Ali Yerlikaya, milletvekilleri, Büyükşehir Belediye Başkanı Fatma Şahin ile birlikte BakanTüfenkci'nin programına eşlik eden GSO Başkanı Adil Konukoğlu da saldırının yaşandığı sokakta incelemelerde bulundu. Daha sonra Beybahçe Sosyal Tesisleri'ndeki taziye evine geçen Konkoğlu, terör eyleminde yakınlarını kaybeden ailelere başsağlığı diledi. temennileri iletildi, dualar okundu.Gelin ve damadın ailesiyle de görüşerek üzüntülerini dile getiren GSO Başkanı Adil Konukoğlu , acılarını yürekten hissettiklerini belirterek, "Saldırıda hayatını kaybeden vatandaşlarımıza bir kez daha Allah'tan rahmet, yaralılara acil şifalar diliyorum. Onların her birisi bizim evladımızdı. Üzüntünüzü paylaşıyor, acınızı anlıyoruz. İnşallah bu zor günleri de birlik beraberlik içinde atlatacağız" dedi. - GAZİANTEP 
`
	doc, _ := parseData("testData/xml/valid.xml")
	if doc.Type != "mainstream" || doc.Forum != "forum" || doc.ForumTitle != "forumtitle" || doc.Language != "turkish" || doc.GMTOffset != "-8" || doc.DiscussionTitle != "Konkoğlu'ndan Taziya Ziyareti" || doc.TopicText != topicText || doc.SpamScore != "0.20" || doc.PostNum != "1" || doc.PostID != "post-1" || doc.PostURL != "http://www.haberler.com/konkoglu-ndan-taziya-ziyareti-8731165-haberi/" || doc.PostDate != "20160826" || doc.PostTime != "time" || doc.Username != "username" || doc.Post != "post" || doc.Signature != "\nsignature\n" || doc.ExternalLinks != "\nlinks\n" || doc.Country != "TR" || doc.MainImage != "http://img.haberler.com/haber/165/konkoglu-ndan-taziya-ziyareti-8731165_ov.jpg" {
		t.Error("expected match")
	}

	// invalid document
	_, err := parseData("testData/xml/invalid.xml")
	if err == nil {
		t.Error("expected error")
	}

	// invalid path
	_, err = parseData("testData/xml/!@#$%^&*()_+?")
	if err == nil {
		t.Error("expected error")
	}
}

func TestWriteToRedisRealConnection(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	pool = newPool("localhost", 6379, "", 80, 12000)
	err := writeToRedis(&document{XML: "<xmlData>hello world</xmlData>", PostURL: "http://myurl.com/post1234"})
	if err != nil {
		t.Error("expected success", err)
	}

	err = writeToRedis(&document{XML: "<xmlData>new world</xmlData>", PostURL: "http://myurl.com/post9876"})
	if err != nil {
		t.Error("expected success", err)
	}

	err = writeToRedis(&document{XML: "<xmlData>updated world</xmlData>", PostURL: "http://myurl.com/post1234"})
	if err != nil {
		t.Error("expected success", err)
	}

	c := pool.Get()
	defer c.Close()
	// check list length
	l, _ := redis.Int(c.Do("LLEN", "NEWS_XML"))
	if l != 2 {
		t.Error("expected list length of 2", l)
	}

	// check values in list
	vals, _ := redis.Strings(c.Do("LRANGE", "NEWS_XML", 0, 2))
	if len(vals) != 2 || vals[0] != "<xmlData>new world</xmlData>" || vals[1] != "<xmlData>updated world</xmlData>" {
		t.Error("expected values", vals)
	}

	// check values in key value store
	v1, _ := redis.String(c.Do("GET", "http://myurl.com/post1234"))
	v2, _ := redis.String(c.Do("GET", "http://myurl.com/post9876"))
	if v1 != "<xmlData>updated world</xmlData>" || v2 != "<xmlData>new world</xmlData>" {
		t.Error("expected correct key value values", v1, v2)
	}
}
