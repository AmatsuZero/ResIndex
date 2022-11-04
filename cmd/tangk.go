package cmd

import (
	"ResIndex/dao"
	"ResIndex/utils"
	"context"
	"database/sql"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/jamesnetherton/m3u"
	"github.com/spf13/cobra"
	"log"
	"net/url"
	"sync"
	"time"
)

var tankHost *url.URL

const tangkBaseUrl = "index.php/index/index/page"

type TankModel struct {
	dao.M3U8Resource
	UpdateTime time.Time
}

func getTankPageLinks(ctx context.Context, start int) (err error) {
	log.SetPrefix("Tank ")

	tankHost, err = url.Parse("https://vip.tangk2.com")
	if err != nil {
		return err
	}

	page := start
	u := tankHost.JoinPath(tangkBaseUrl, fmt.Sprintf("%v.html", page))

	shouldStop := false
	for !shouldStop {
		var models []*TankModel
		log.Printf("第%v页解析开始\n", page)
		utils.GetDocument(u.String(), func(doc *goquery.Document) { // 解析有多少页
			loc, _ := time.LoadLocation("Asia/Shanghai")
			const form = "2006-01-02 15:04:05"
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
					model := &TankModel{}
					model.Ref = sql.NullString{String: link, Valid: true}
					donCreate := !dao.Any(model, "ref = ?", link) && len(model.Url) > 0
					name := td.Text()
					tag := selection.Find(fmt.Sprintf("body > main > div > div > a > table > tbody > tr:nth-child(%v) > td:nth-child(3)", i+1)).Text()
					model.Name = sql.NullString{String: name, Valid: true}
					model.Ref = sql.NullString{String: link, Valid: true}
					model.Tags = []sql.NullString{{tag, true}}
					if !donCreate {
						dao.Create(model)
					}

					// 提取时间
					t := doc.Find(fmt.Sprintf("body > main > div > div > a > table > tbody > tr:nth-child(%v) > td:nth-child(5) > span", i+1)).Text()
					tt, e := time.ParseInLocation(form, t, loc)
					if e == nil {
						model.UpdateTime = tt
					}
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
		updateTankDetailPages(ctx, models)
		// 保存
		dao.DB.Save(&models)
	}
	log.Printf("解析结束，一共解析:%v页", page)
	return nil
}

// 更新详情页
func updateTankDetailPages(ctx context.Context, models []*TankModel) {
	concurrent := ctx.Value("concurrent").(int)
	ch := make(chan struct{}, concurrent)
	wg := &sync.WaitGroup{}

	for _, model := range models {
		if !model.Ref.Valid {
			continue
		}

		wg.Add(1)
		ch <- struct{}{}

		go func(m *TankModel) {
			defer wg.Done()
			utils.GetDocument(m.Ref.String, func(doc *goquery.Document) { // 找到 m3u8 资源链接
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

func exportTankPagesList(output string) {
	var records []*TankModel
	dao.DB.Find(&records)

	playlist := &m3u.Playlist{}

	for _, record := range records {
		track := m3u.Track{
			URI: record.Url,
		}
		if record.Name.Valid {
			track.Name = record.Name.String
		}
		var tags []m3u.Tag
		for _, tag := range record.Tags {
			if tag.Valid {
				tags = append(tags, m3u.Tag{
					Name:  "分类",
					Value: tag.String,
				})
			}
		}
		track.Tags = tags
		playlist.Tracks = append(playlist.Tracks, track)
	}

	err := utils.Export(output, playlist)
	if err != nil {
		log.Fatalf("生成 m3u 文件失败: %v", err)
	} else {
		log.Printf("生成 m3u 文件到 %v", output)
	}
}

func Tank() *cobra.Command {
	page, cnt := new(int), new(int)
	migrate := func(cmd *cobra.Command, args []string) {
		err := dao.DB.AutoMigrate(&TankModel{})
		if err != nil {
			log.Panicf("自动迁移失败: %v", err)
		}
	}

	ctx := context.Background()
	cmd := &cobra.Command{
		Use:    "tank",
		Short:  "坦克资源网资源爬取",
		PreRun: migrate,
		Run: func(cmd *cobra.Command, args []string) {
			ctx = context.WithValue(ctx, "concurrent", *cnt)
			err := getTankPageLinks(ctx, *page)
			if err != nil {
				log.Fatalf("解析失败 : %v", err)
			}
		},
	}

	output := ""
	exportCmd := &cobra.Command{
		Use:    "export",
		Short:  "导出为 m3u 格式",
		PreRun: migrate,
		Run: func(cmd *cobra.Command, args []string) {
			exportTankPagesList(output)
		},
	}
	exportCmd.Flags().StringVarP(&output, "output", "o", "", "导出路径")
	_ = exportCmd.MarkFlagRequired("output")
	cnt = cmd.Flags().IntP("concurrent", "c", 10, "指定并发数量")
	page = cmd.Flags().IntP("page", "p", 1, "指定起始页码")
	cmd.AddCommand(exportCmd)

	return cmd
}
