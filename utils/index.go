package utils

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/jamesnetherton/m3u"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
)

var Client *http.Client

func SetupWebClient() {
	Client = &http.Client{}
}

func GetDocument(url string, extractor ...func(doc *goquery.Document)) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.63 Safari/537.36")
	req.Header.Add("Referer", req.URL.Scheme+"://"+req.URL.Host+"/")

	res, err := Client.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(res.Body)

	if res.StatusCode != 200 {
		return fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return err
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

	return nil
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

func Cmd(commandName string, params []string) (string, error) {
	cmd := exec.Command(commandName, params...)
	//fmt.Println("Cmd", cmd.Args)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return "", err
	}
	err = cmd.Wait()
	return out.String(), err
}

func IsPathExist(p string) bool {
	_, err := os.Stat(p)
	return errors.Is(err, os.ErrNotExist)
}

func MakeDirSafely(p string) error {
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		err = os.Mkdir(p, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}
