package sis001

import (
	"ResIndex/dao"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"log"
	"path"
	"strings"
)

type Detail struct {
	ThreadInfo
	DocRequester
	Category string
	Host     string
}

func (d *Detail) ExtractInfo() (*InfoModel, error) {
	doc, err := d.GetDoc(d.href)
	if err != nil {
		return nil, err
	}
	model := &InfoModel{}
	model.Category = d.Category
	model.Tags = d.tag

	msgFont := doc.Find("div.t_msgfont").First()
	id, _ := msgFont.Attr("id")
	model.PostId = strings.Split(id, "_")[1]

	sel := "#postmessage_" + model.PostId
	text := doc.Find(sel).Text()
	model.FillInfo(strings.Split(text, "\n"))

	id = d.href
	ext := path.Ext(id)
	idx := strings.LastIndex(id, ext)
	id = id[:idx]
	model.ThreadId = strings.Split(id, "-")[1]

	if len(model.Name.String) == 0 || model.Name.String == "---" {
		model.ExtractNewListModelTitle(doc)
	}
	model.TorrentLink, err = d.extractNewListModelTorrentLink(doc, model.PostId)
	dao.DB.Create(model)
	return model, err
}

func (d *Detail) extractNewListModelTorrentLink(doc *goquery.Document, postId string) (string, error) {
	sel := "#pid" + postId + " > tbody > tr:nth-child(1) > "
	sel += "td.postcontent > div.postmessage.defaultpost > "
	sel += "div.box.postattachlist > dl.t_attachlist > dt > a"
	link, ok := doc.Find(sel).FilterFunction(func(i int, selection *goquery.Selection) bool {
		href, _ := selection.Attr("href")
		return strings.Contains(href, "attachment.php")
	}).Attr("href")
	if !ok {
		return "", nil
	}
	return d.getTorrentLink(link)
}

func (d *Detail) getTorrentLink(url string) (string, error) {
	url = d.Host + "/bbs/" + url
	log.Printf("🔗 即将提取种子链接: %v", url)
	doc, err := d.GetDoc(url)
	if err != nil {
		return "", err
	}
	link, ok := doc.Find("#downloadBtn").Attr("href")
	if !ok {
		return "", fmt.Errorf("种子链接没有找到：%v", url)
	}
	url = d.href + link
	log.Printf(`🍺 找到种子链接：%v`, url)
	return url, nil
}
