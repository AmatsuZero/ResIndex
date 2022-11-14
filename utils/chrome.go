package utils

import (
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"log"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"time"
)

func GetDocumentByChrome(url string, extractor ...func(doc *goquery.Document)) error {
	html, err := VisitWebPage(url)
	if err != nil {
		return err
	}

	document, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return err
	}

	wg := &sync.WaitGroup{}
	for _, f := range extractor {
		wg.Add(1)
		go func(ff func(doc *goquery.Document)) {
			ff(document)
			wg.Done()
		}(f)
	}

	wg.Wait()

	return nil
}

func VisitWebPageWithActions(host string, actions ...chromedp.Action) (err error) {
	options := []chromedp.ExecAllocatorOption{
		chromedp.Flag("headless", true), // debug使用
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.UserAgent(`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.63 Safari/537.36`),
		chromedp.Flag("mute-audio", false), // 关闭声音
		//启动chrome 不适用沙盒, 性能优先
		chromedp.Flag("no-sandbox", true),
		//启动chrome的时候不检查默认浏览器
		chromedp.Flag("no-default-browser-check", true),
	}
	//初始化参数，先传一个空的数据
	options = append(chromedp.DefaultExecAllocatorOptions[:], options...)

	c, _ := chromedp.NewExecAllocator(context.Background(), options...)

	// create context
	chromeCtx, cancel := chromedp.NewContext(c)
	defer cancel()
	// 执行一个空task, 用提前创建Chrome实例
	var res string
	err = chromedp.Run(chromeCtx, setHeaders(
		host,
		map[string]interface{}{
			"Accept-Language": "zh-cn,zh;q=0.5",
			"X-Forwarded-For": genIpaddr(),
		},
		&res,
	))

	if err != nil {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(chromeCtx, 1000*time.Second)
	defer cancel()

	err = chromedp.Run(timeoutCtx, actions...)

	if err != nil {
		log.Printf("Run err : %v\n\n", err)
		_ = chromedp.Cancel(timeoutCtx)
		return
	}
	return
}

func VisitWebPage(u string) (html string, err error) {
	addr, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	acts := []chromedp.Action{
		chromedp.Navigate(addr.String()),
		chromedp.Sleep(200 * time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			node, err := dom.GetDocument().Do(ctx)
			if err != nil {
				return err
			}
			html, err = dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)
			return err
		}),
	}

	err = VisitWebPageWithActions(fmt.Sprintf("%v://%v", addr.Scheme, addr.Host), acts...)
	return
}

// setHeaders returns a task list that sets the passed headers.
func setHeaders(host string, headers map[string]interface{}, res *string) chromedp.Tasks {
	return chromedp.Tasks{
		network.Enable(),
		network.SetExtraHTTPHeaders(headers),
		chromedp.Navigate(host),
		chromedp.Text(`#result`, res, chromedp.ByID, chromedp.NodeVisible),
	}
}

// 生成随机 IP 地址
func genIpaddr() string {
	rand.Seed(time.Now().Unix())
	ip := fmt.Sprintf("%d.%d.%d.%d", rand.Intn(255), rand.Intn(255), rand.Intn(255), rand.Intn(255))
	return ip
}
