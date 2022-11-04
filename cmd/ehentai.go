package cmd

import (
	"ResIndex/dao"
	"ResIndex/utils"
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
	"log"
	"sync"
)

const ehentaiBase = "https://e-hentai.org"

type EhentaiBook struct {
	dao.M3U8Resource
	TorrentLinkRef string
	TorrentLinks   []EHentaiTorrent `gorm:"foreignkey:TorrentLinkRef"`
	Pages          []EHentaiTorrent `gorm:"foreignkey:Url"`
}

type EHentaiTorrent struct {
	gorm.Model
	TorrentLinkRef string
	Size           int64
	Link           string
}

type EHentaiComicPage struct {
	gorm.Model
	PageRef string
	Link    string
	ID      int
}

func (b *EhentaiBook) Download(output string) error {
	return nil
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
	u := fmt.Sprintf("%v/?page=%v", ehentaiBase, start)
	hasNextPage := true

	for hasNextPage {
		hasNextPage = extractEHentaiRefLinksOnPage(ctx, u)
	}

	log.Printf("提取结束")
}

func extractEHentaiRefLinksOnPage(ctx context.Context, u string) (hasNextPage bool) {
	var models []*EhentaiBook
	e := utils.GetDocument(u, func(doc *goquery.Document) {
		model := &EhentaiBook{}
		models = append(models, model)
	})

	if e != nil {
		log.Printf("提取页面内容出错, %v", e)
		return
	}

	log.Printf("提取链接结束，一共提取 %v 个链接，准备提取内容...\n", len(models))
	updateEHentaiBookDetail(ctx, models)
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
			defer wg.Done()
			e := utils.GetDocument(mm.Ref.String, func(doc *goquery.Document) {

			})
			if e != nil {
				log.Printf("提取页面内容出错: %v", e)
			}
			<-ch
		}(model)
	}

	dao.DB.Save(models)
}
