package utils

import (
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/http"
	"sync"
)

func GetDocument(url string, extractor ...func(doc *goquery.Document)) {
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
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
