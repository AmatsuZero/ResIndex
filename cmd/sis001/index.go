package sis001

import (
	"context"
	"github.com/PuerkitoBio/goquery"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type PageType string
type CtxKey string

const (
	NEW     PageType = "forum-561"
	ACG     PageType = "forum-231"
	NOVEL   PageType = "forum-383"
	WESTERN PageType = "forum-229"
	INDEX   PageType = "-"

	HostKey       CtxKey = "host"
	StartTimeKey  CtxKey = "start-time"
	PathDictKey   CtxKey = "sis-paths"
	ConcurrentKey CtxKey = "concurrent"
)

func PreRun(cmd *cobra.Command) {
	log.SetPrefix("sis ")

	hosts := []string{
		// "https://sis001.com/",
		"http://154.84.6.38/",
		"http://162.252.9.11/",
		"http://154.84.5.249/",
		"http://154.84.5.211/",
		"http://162.252.9.2/",
		"http://68.168.16.150/",
		"http://68.168.16.151/",
		"http://68.168.16.153/",
		"http://68.168.16.154/",
		"http://23.225.255.95/",
		"http://23.225.255.96/",
		"https://pux.sisurl.com/",
		"http://23.225.172.96/",
	}

	sisPaths := map[PageType]string{
		INDEX:   "bbs",
		NEW:     "bbs/" + string(NEW),
		ACG:     "bbs/" + string(ACG),
		NOVEL:   "bbs/" + string(NOVEL),
		WESTERN: "bbs/" + string(WESTERN),
	}

	host := findAvailableHost(hosts, sisPaths)
	if len(host) == 0 {
		log.Fatalf("❌ 无可用的 Host")
	}

	now := time.Now().Local()
	log.Printf("🚀 启动任务：%v\n", now.Format(time.UnixDate))

	ctx := cmd.Context()
	ctx = context.WithValue(ctx, HostKey, host)
	ctx = context.WithValue(ctx, StartTimeKey, now)
	ctx = context.WithValue(ctx, PathDictKey, sisPaths)

	cmd.SetContext(ctx)
}

func findAvailableHost(hosts []string, dict map[PageType]string) string {
	for _, host := range hosts {
		if checkIsHostAvailable(dict, host) {
			return host
		}
	}
	return ""
}

func checkIsHostAvailable(dict map[PageType]string, host string) bool {
	bbs := host + dict[INDEX]
	res, err := http.Get(bbs)
	if err != nil {
		log.Printf("❌ 访问 Host 地址出错：%v\n", err)
		return false
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	if res.StatusCode != 200 {
		log.Printf("status code error: %d %s\n", res.StatusCode, res.Status)
		return false
	}
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Println(err)
		return false
	}
	txt := doc.Find("title").Text()
	txt = strings.Replace(txt, "  ", " ", -1)
	txt = strings.TrimSpace(txt)
	if strings.Contains(txt, "SiS001! Board -") {
		return true
	}
	return false
}

func createClient() *resty.Client {
	// 参考：https://www.loginradius.com/blog/engineering/tune-the-go-http-client-for-high-performance/
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxConnsPerHost = 100
	t.MaxIdleConnsPerHost = 100

	client := resty.New()
	client.SetDebug(false)
	client.SetTransport(t)
	client.SetRetryCount(3)
	client.SetTimeout(3 * time.Minute)

	return client
}
