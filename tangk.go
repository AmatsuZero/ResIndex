package main

import (
	"database/sql"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/url"
	"sync"
)

var tankHost *url.URL

const tangkBaseUrl = "index.php/index/index/page"

func getTankPageLinks(start int) (err error) {
	log.SetPrefix("Tank ")

	tankHost, err = url.Parse("https://vip.tangk2.com")
	if err != nil {
		return err
	}

	page := start
	u := tankHost.JoinPath(tangkBaseUrl, fmt.Sprintf("%v.html", page))

	shouldStop := false
	for !shouldStop {
		var models []*M3U8Resource
		log.Printf("第%v页解析开始\n", page)
		getDocument(u.String(), func(doc *goquery.Document) { // 解析有多少页
			// 找到 tbbody
			doc.Find("body > main > div > div > a > table > tbody").
				Children().
				Each(func(i int, selection *goquery.Selection) {
					td := selection.Find("td.yp").Find("a")
					link, ok := td.Attr("href")
					if !ok {
						return
					}
					link = tankHost.JoinPath(link).String()
					model := &M3U8Resource{Ref: sql.NullString{String: link, Valid: true}}
					if !model.Any("ref = ?", link) {
						log.Printf("有数据，跳过: %v", link)
						return
					}
					if len(model.Url) > 0 {
						log.Printf("有数据，跳过: %v", link)
						return
					}

					name := td.Text()
					tag := selection.Find(fmt.Sprintf("body > main > div > div > a > table > tbody > tr:nth-child(%v) > td:nth-child(3)", i+1)).Text()
					model.Create(M3U8Resource{
						Name: sql.NullString{String: name, Valid: true},
						Ref:  sql.NullString{String: link, Valid: true},
						Tags: []sql.NullString{{tag, true}},
					})

					models = append(models, model)
				})
		},
			func(doc *goquery.Document) {
				// 查找下一页
				link, ok := doc.Find("body > main > div > div > div > ul.pag-list > li.next.pag-active > a").Attr("href")
				if !ok {
					shouldStop = true
				} else {
					u = tankHost.JoinPath(link)
					page++
				}
			})
		updateDetails(models)
		// 保存
		DB.Save(&models)
	}
	log.Printf("解析结束，一共解析:%v页", page)
	return nil
}

// 更新详情页
func updateDetails(models []*M3U8Resource) {
	ch := make(chan struct{}, 10)
	wg := &sync.WaitGroup{}

	for _, model := range models {
		if !model.Ref.Valid {
			continue
		}

		wg.Add(1)
		ch <- struct{}{}
		
		go func(m *M3U8Resource) {
			defer wg.Done()
			getDocument(m.Ref.String, func(doc *goquery.Document) { // 找到 m3u8 资源链接
				val, ok := doc.Find("body > main > section.dy-collect > div > div:nth-child(2) > ul > li > a:nth-child(4)").Attr("href")
				if ok {
					m.Url = val
				}
			}, func(doc *goquery.Document) { // 找到预览图
				img, ok := doc.Find("body > main > section.dy-ins > div > div.detailed > div > div.dy-photo > img").Attr("src")
				if ok {
					m.Thumbnail = sql.NullString{String: tankHost.JoinPath(img).String(), Valid: true}
				}
			})
			<-ch
		}(model)
	}
	wg.Wait()
}
