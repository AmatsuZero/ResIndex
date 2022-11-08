package sis001

import (
	"ResIndex/dao"
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"io"
	"log"
	"path"
	"strconv"
	"strings"
	"sync"
)

func NewArticle() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nl",
		Short: "新作品",
		Run:   createNewList,
	}
	return cmd
}

type ThreadInfo struct {
	href, tag string
}

type DocRequester struct {
	client *resty.Client
	ctx    context.Context
}

func (d *DocRequester) GetDoc(url string) (*goquery.Document, error) {
	resp, err := d.client.R().
		SetContext(d.ctx).
		SetDoNotParseResponse(true).
		Get(url)
	if err != nil {
		return nil, err
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(resp.RawBody())
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("status code error: %d %s", resp.StatusCode(), resp.Status())
	}
	return goquery.NewDocumentFromReader(resp.RawBody())
}

func (t *ThreadInfo) parseNewListData() {
	log.Printf("🔍 即将解析新作品详情页面：%v", t.href)
}

func (t *ThreadInfo) String() string {
	return fmt.Sprintf("【标签】%v 【链接】%v", t.tag, t.href)
}

type ThreadPage struct {
	DocRequester
	host, title     string
	PageType        PageType
	MaxPageSelector string
}

func (p *ThreadPage) GetAllThreadsOnCurrentPage() error {
	return nil
}

func (p *ThreadPage) CurrentPageURL(cur int) string {
	dict := p.ctx.Value(PathDictKey).(map[PageType]string)
	sprintf := fmt.Sprintf("%v-%v.html", dict[p.PageType], cur)
	return p.host + sprintf
}

func (p *ThreadPage) FindMaxPage(_ *goquery.Document) int {
	return 0
}

type NewList struct {
	ThreadPage
}

func createNewList(cmd *cobra.Command, _ []string) {
	host := cmd.Context().Value(HostKey).(string)
	n := &NewList{}
	n.host = host
	n.PageType = NEW
	n.title = "新作品"
	n.ctx = cmd.Context()
	n.MaxPageSelector = "#wrapper > div:nth-child(1) > div:nth-child(9) > div > a.last"
	n.client = createClient()
	err := n.ExtractInfo()
	if err != nil {
		log.Fatalf("提取新作品页面失败：%v", err)
	}
}

func (n *NewList) FindMaxPage(doc *goquery.Document) int {
	link, ok := doc.Find(n.MaxPageSelector).Attr("href")
	maxPage := 0
	if ok {
		ext := path.Ext(link)
		idx := strings.LastIndex(link, ext)
		link = link[:idx]
		link = strings.ReplaceAll(link, string(n.PageType)+"-", "")
		maxPage, _ = strconv.Atoi(link)
	}
	return maxPage
}

func (n *NewList) GetAllThreadsOnCurrentPage(cur int) ([]ThreadInfo, error) {
	url := n.CurrentPageURL(cur)
	doc, err := n.GetDoc(url)
	log.Printf("🔗 即将打开%v第%v页\n", n.title, cur)
	if err != nil {
		return nil, err
	}
	var tInfos []ThreadInfo
	sel := "#wrapper > div:nth-child(1) > div.mainbox.threadlist > form"
	doc.Find(sel).Find("tbody[id]").FilterFunction(func(i int, selection *goquery.Selection) bool {
		id, ok := selection.Attr("id")
		if !ok {
			return false
		}
		return strings.HasPrefix(id, "normalthread_")
	}).Each(func(i int, selection *goquery.Selection) {
		tag := selection.Find("th > em > a").Text()
		href, _ := selection.Find("th > span > a").Attr("href")
		href = n.host + "bbs/" + href
		tInfos = append(tInfos, ThreadInfo{href, tag})
	})

	return tInfos, nil
}

func (n *NewList) ExtractInfo() error {
	url := n.CurrentPageURL(1)
	doc, err := n.GetDoc(url)
	if err != nil {
		return err
	}
	// 先找到最大页码
	maxPage := n.FindMaxPage(doc)

	wg := sync.WaitGroup{}
	ch := n.ctx.Value(ConcurrentKey).(chan struct{})

	for i := 1; i <= maxPage; i++ {
		wg.Add(1)
		ch <- struct{}{} // 写一个标记到 chan，chan缓存满时会阻塞

		go func(cur int) {
			defer func() {
				wg.Done() // 将计数减1
				<-ch      // 读取chan
			}()
			infos, err := n.GetAllThreadsOnCurrentPage(cur)
			if err != nil {
				log.Printf("❌ 解析新作品页面第%v出错：%v", cur, err)
				return
			}
			log.Printf(`🔗 开始解析新作品页面第%v页`, cur)
			models := n.extractDetails(infos)
			dao.DB.Save(models)
			log.Printf(`🍺 解析新作品页面第%v页完成`, cur)
		}(i)
	}
	wg.Wait()

	return nil
}

func (n *NewList) extractDetails(infos []ThreadInfo) []*InfoModel {
	wg := sync.WaitGroup{}

	output := make([]*InfoModel, 0)
	var lock sync.Mutex

	for _, info := range infos {
		wg.Add(1)

		go func(ti ThreadInfo) {
			defer wg.Done()
			model, err := n.extractDetail(ti)
			if err != nil {
				log.Printf("❌ 解析新作品详情页面出错：%v\n", err)
			} else if model != nil {
				lock.Lock()
				output = append(output, model)
				lock.Unlock()
				log.Printf(`🍺 解析完成: %v-%v`, ti.tag, model.Title)
			}
		}(info)
	}

	wg.Wait()
	return output
}

func (n *NewList) extractDetail(info ThreadInfo) (*InfoModel, error) {
	detail := Detail{
		ThreadInfo: info,
		Category:   "new",
		Host:       n.host,
	}
	detail.ctx = n.ctx
	detail.client = n.client
	return detail.ExtractInfo()
}
