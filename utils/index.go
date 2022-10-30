package utils

import (
	"bufio"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/jamesnetherton/m3u"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

func GetDocument(url string, extractor ...func(doc *goquery.Document)) {
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(res.Body)
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return
	}

	wg := &sync.WaitGroup{}
	for _, f := range extractor {
		wg.Add(1)
		go func(f func(doc *goquery.Document)) {
			f(doc)
			defer wg.Done()
		}(f)
	}
	wg.Wait()
}

func Export(output string, playlist *m3u.Playlist) error {
	fo, err := os.Create(output)
	if err != nil {
		return err
	}

	defer func(fo *os.File) {
		err = fo.Close()
		if err != nil {
			log.Panicf("关闭文件失败: %v", err)
		}
	}(fo)

	w := bufio.NewWriter(fo)
	err = m3u.MarshallInto(*playlist, w)
	return err
}
