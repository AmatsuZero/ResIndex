package cmd

import (
	"ResIndex/dao"
	"ResIndex/utils"
	"context"
	"database/sql"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

const ehentaiBase = "https://e-hentai.org"

type EhentaiBook struct {
	dao.M3U8Resource
	TorrentLinkRef string
	TorrentLinks   []EHentaiTorrent   `gorm:"foreignkey:TorrentLinkRef"`
	Pages          []EHentaiComicPage `gorm:"foreignkey:ID"`
	PublishTime    time.Time
	Language, Parody, Uploader,
	Character, Female, Artist string
	TotalPages       int
	FileSize         int64
	OriginalLanguage string
}

func (b *EhentaiBook) extract() {
	log.Printf("准备提取详情:%v", b.Name)
	if e := b.extractDetails(); e != nil {
		log.Printf("提取详情信息失败: %v\n", e)
	} else {
		log.Println("提取详情结束")
	}
	log.Printf("准备提取种子:%v", b.Name)
	if e := b.extractTorrent(); e != nil {
		log.Printf("提取种子信息失败: %v\n", e)
	} else {
		log.Println("提取种子结束")
	}
}

func (b *EhentaiBook) extractDetails() error {
	e := utils.GetDocumentByChrome(b.Ref.String, func(doc *goquery.Document) {
		length := doc.Find("#gdd > table > tbody > tr:nth-child(6) > td.gdt2").Text()
		if len(length) > 0 {
			arr := strings.Split(length, " ")
			if len(arr) > 0 {
				num, e := strconv.Atoi(arr[0])
				if e == nil {
					b.TotalPages = num
				}
			}
		}

		fileSize := doc.Find("#gdd > table > tbody > tr:nth-child(5) > td.gdt2").Text()
		size, e := utils.FromHumanSize(fileSize)
		if e != nil {
			b.FileSize = size
		}

		lan := doc.Find("#gdd > table > tbody > tr:nth-child(4) > td.gdt2").Text()
		if len(lan) > 0 {
			arr := strings.Split(lan, " ")
			b.Language = arr[0]
		}

		category := doc.Find("#gdc > div").Text()
		b.Tags = category

		dict := map[string]EHentaiComicPage{}
		for _, p := range b.Pages {
			dict[p.Ref] = p
		}

		doc.Find("#gdt").Children().Each(func(i int, selection *goquery.Selection) {
			var page EHentaiComicPage
			href := selection.Find(fmt.Sprintf("#gdt > div:nth-child(%v) > div > a", i+1))
			link, ok := href.Attr("href")

			needCreate := false
			if !ok {
				return
			}

			_, ok = dict[link]
			if ok {
				page = dict[link]
			} else {
				page = EHentaiComicPage{
					ID:  b.ID,
					Ref: link,
				}
				needCreate = true
				b.Pages = append(b.Pages, page)
			}

			if needCreate {
				dao.Create(page)
			}
			page.Index = i
		})
	})

	// 提取图片链接
	for _, page := range b.Pages {
		if err := page.extractLink(); err != nil {
			log.Printf("提取图片链接失败: %v", err)
		}
	}
	return e
}

func (b *EhentaiBook) extractTorrent() error {
	if len(b.TorrentLinkRef) == 0 { // 没有种子
		log.Println("无种子，跳过")
		return nil
	}

	dict := map[string]EHentaiTorrent{}
	for _, link := range b.TorrentLinks {
		dict[link.Link] = link
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	const form = "2006-01-02 15:04"

	return utils.GetDocumentByChrome(b.TorrentLinkRef, func(doc *goquery.Document) {
		doc.Find("#torrentinfo > div > form").Each(func(i int, selection *goquery.Selection) {
			l, ok := selection.Find("div > table > tbody > tr:nth-child(3) > td > a").Attr("href")
			if !ok {
				return
			}
			var link EHentaiTorrent
			needCreate := false
			_, ok = dict[l]
			if ok {
				link = dict[l]
			} else {
				needCreate = true
				link = EHentaiTorrent{Link: l, TorrentLinkRef: b.TorrentLinkRef}
			}

			if needCreate {
				dao.Create(link)
				b.TorrentLinks = append(b.TorrentLinks, link)
			}

			txt := selection.Find("div > table > tbody > tr:nth-child(1) > td:nth-child(1)").Text()
			idx := strings.Index(txt, ":")
			txt = strings.TrimSpace(txt[idx+1:])
			tt, e := time.ParseInLocation(form, txt, loc)
			if e == nil {
				link.UpdatedAt = tt
			}

			txt = selection.Find("#torrentinfo > div > form:nth-child(3) > div > table > tbody > tr:nth-child(1) > td:nth-child(2)").Text()
			idx = strings.Index(txt, ":")
			txt = strings.TrimSpace(txt[idx+1:])
			size, e := utils.FromHumanSize(txt)
			if e == nil {
				link.Size = size
			}
		})
	})
}

type EHentaiTorrent struct {
	gorm.Model
	TorrentLinkRef string
	Size           int64
	Link           string
	Posted         time.Time
	Uploader       string
}

type EHentaiComicPage struct {
	gorm.Model
	//大画廊页面
	Ref string
	// 页面下载链接
	Link string
	// 漫画ID
	ID uint
	// 第几页
	Index int
}

func (p *EHentaiComicPage) extractLink() error {
	return utils.GetDocumentByChrome(p.Ref, func(doc *goquery.Document) {
		link, ok := doc.Find("#img").Attr("src")
		if ok {
			p.Link = link
		}
	})
}

func (b *EhentaiBook) extractTags(sel *goquery.Selection) {
	var language, parody, character, female, artist []string

	sel.Children().Each(func(i int, s *goquery.Selection) {
		txt, ok := s.Attr("title")
		if !ok {
			return
		}
		parts := strings.Split(txt, ":")
		if len(parts) == 0 {
			return
		}
		switch parts[0] {
		case "language":
			language = append(language, parts[1])
		case "parody":
			parody = append(parody, parts[1])
		case "character":
			character = append(character, parts[1])
		case "female":
			character = append(character, parts[1])
		case "artist":
			artist = append(artist, parts[1])
		}
	})

	b.Character = strings.Join(character, ",")
	b.Language = strings.Join(language, ",")
	b.Female = strings.Join(female, ",")
	b.Parody = strings.Join(parody, ",")
	b.Artist = strings.Join(artist, ",")
}

func EHentai() *cobra.Command {
	page, concurrent := new(int), new(int)
	ctx := context.Background()

	migrate := func(cmd *cobra.Command, args []string) {
		err := dao.DB.AutoMigrate(&EhentaiBook{})
		if err != nil {
			log.Panicf("自动迁移失败: %v", err)
		}
	}

	cmd := &cobra.Command{
		Use:    "ehentai",
		Short:  "e 绅士资源爬取",
		PreRun: migrate,
		Run: func(cmd *cobra.Command, args []string) {
			log.SetPrefix("ehentai ")
			ctx = context.WithValue(ctx, concurrentKey, *concurrent)
			scrapeEhentai(ctx, *page)
		},
	}

	concurrent = cmd.Flags().IntP(string(concurrentKey), "c", 10, "指定并发数量")
	page = cmd.Flags().IntP("page", "p", 1, "指定起始页码")

	return cmd
}

func scrapeEhentai(ctx context.Context, start int) {
	u := ehentaiBase
	if start > 1 {
		u = fmt.Sprintf("%v/?page=%v", ehentaiBase, start)
	}

	nextPage := u
	for len(nextPage) > 0 {
		nextPage = extractEHentaiRefLinksOnPage(ctx, nextPage)
	}

	log.Printf("提取结束")
}

func extractEHentaiRefLinksOnPage(ctx context.Context, u string) (nextPage string) {
	var models []*EhentaiBook
	loc, _ := time.LoadLocation("Asia/Shanghai")
	const form = "2006-01-02 15:04"

	log.Printf("准备提取链接 %v\n", u)
	e := utils.GetDocumentByChrome(u, func(doc *goquery.Document) {
		log.Println("开始提取")
		grid := doc.Find("body > div.ido > div:nth-child(2) > table.itg.gltc > tbody")
		grid.Children().Each(func(i int, selection *goquery.Selection) {
			if i == 0 { // 跳过表头
				return
			}
			model := &EhentaiBook{}
			href, ok := selection.Find("td.gl3c.glname > a").Attr("href")
			if ok {
				model.Ref = sql.NullString{
					String: href,
					Valid:  true,
				}
			}

			donCreate := !dao.NotExist(model, "ref = ?", href) && len(model.Pages) > 0

			sel := selection.Find("td.gl2c > div.glthumb > div:nth-child(1) > img")
			link, ok := sel.Attr("src")
			if ok {
				model.Thumbnail = sql.NullString{
					String: link,
					Valid:  true,
				}
			}

			name, ok := sel.Attr("title")
			if ok {
				model.Name = sql.NullString{
					String: name,
					Valid:  true,
				}
			}

			if !donCreate {
				dao.Create(model)
			}

			t := selection.Find("td.gl2c > div:nth-child(3) > div:nth-child(1)").Text()
			tt, e := time.ParseInLocation(form, t, loc)
			if e == nil {
				model.PublishTime = tt
			}

			uploader := selection.Find("td.gl4c.glhide > div:nth-child(1) > a").Text()
			if len(uploader) > 0 {
				model.Uploader = uploader
			}

			sel = selection.Find("tr:nth-child(2) > td.gl3c.glname > a > div:nth-child(2)")
			model.extractTags(sel)

			torrentRef, ok := selection.Find("td.gl2c > div:nth-child(3) > div.gldown > a").Attr("href")
			if ok {
				model.TorrentLinkRef = torrentRef
			}
			models = append(models, model)

			// 查找下一页
			page, ok := doc.Find("body > div.ido > div:nth-child(2) > table.ptb > tbody > tr > td:nth-child(11) > a").Attr("href")
			if ok {
				nextPage = page
			}
		})
	})

	if e != nil {
		log.Printf("提取页面内容出错, %v", e)
		return
	}

	log.Printf("提取链接结束，一共提取 %v 个链接，准备提取内容...\n", len(models))
	updateEHentaiBookDetail(ctx, models)
	log.Println("提取内容结束，准备提取下一页")
	return
}

func updateEHentaiBookDetail(ctx context.Context, models []*EhentaiBook) {
	if len(models) == 0 {
		return
	}
	concurrent := ctx.Value(concurrentKey).(int)
	ch := make(chan struct{}, concurrent)
	wg := &sync.WaitGroup{}

	for _, model := range models {
		if !model.Ref.Valid {
			continue
		}

		wg.Add(1)
		ch <- struct{}{}

		go func(mm *EhentaiBook) {
			mm.extract()
			<-ch
			wg.Done()
		}(model)
	}

	dao.DB.Save(models)
}
