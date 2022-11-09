package cmd

import (
	"ResIndex/dao"
	"ResIndex/telegram"
	"ResIndex/utils"
	"context"
	"database/sql"
	"fmt"
	"github.com/grafov/m3u8"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/jamesnetherton/m3u"
	"github.com/spf13/cobra"
)

var tankHost *url.URL

const tangkBaseUrl = "index.php/index/index/page"

type ContextKey string

const (
	concurrentKey ContextKey = "concurrent"
)

type TankModel struct {
	dao.M3U8Resource
	UpdateTime time.Time
	Duration   float64
}

func getTankPageLinks(ctx context.Context, start int) (err error) {
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
		e := utils.GetDocument(u.String(), func(doc *goquery.Document) { // 解析有多少页
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
					donCreate := !dao.NotExist(model, "ref = ?", link) && len(model.Url) > 0
					name := td.Text()
					tag := selection.Find(fmt.Sprintf("body > main > div > div > a > table > tbody > tr:nth-child(%v) > td:nth-child(3)", i+1)).Text()
					model.Name = sql.NullString{String: name, Valid: true}
					model.Ref = sql.NullString{String: link, Valid: true}
					model.Tags = tag
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
		if e != nil {
			log.Printf("提取页面内容出错：%v", e)
			continue
		}
		updateTankDetailPages(ctx, models)
		// 保存
		dao.DB.Save(&models)
	}
	log.Printf("解析结束，一共解析:%v页", page)
	return nil
}

// 更新详情页
func updateTankDetailPages(ctx context.Context, models []*TankModel) {
	concurrent := ctx.Value(concurrentKey).(int)
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
			e := utils.GetDocument(m.Ref.String, func(doc *goquery.Document) { // 找到 m3u8 资源链接
				val, ok := doc.Find("body > main > section.dy-collect > div > div:nth-child(2) > ul > li > a:nth-child(4)").Attr("href")
				if ok {
					updateTankPageDuration(val, m)
				}
			}, func(doc *goquery.Document) { // 找到预览图
				img, ok := doc.Find("body > main > section.dy-ins > div > div.detailed > div > div.dy-photo > img").Attr("src")
				if ok {
					m.Thumbnail = sql.NullString{String: tankHost.JoinPath(img).String(), Valid: true}
				}
			})
			if e != nil {
				log.Printf("提取页面出错 %v\n", e)
			}
			<-ch
		}(model)
	}
	wg.Wait()
}

func updateTankPageDuration(u string, model *TankModel) {
	resp, err := http.Get(u)
	if err != nil {
		log.Printf("访问资源地址失败: %v\n", err)
		return
	}

	list, t, err := m3u8.DecodeFrom(resp.Body, false)
	if err != nil {
		log.Printf("解析资源地址失败: %v\n", err)
		return
	}

	if len(model.Url) == 0 {
		model.Url = u
	}
	model.Duration = 0
	if t == m3u8.MEDIA {
		mediaList := list.(*m3u8.MediaPlaylist)
		for _, segment := range mediaList.Segments {
			if segment != nil {
				model.Duration += segment.Duration
			}
		}
	} else if t == m3u8.MASTER {
		masterList := list.(*m3u8.MasterPlaylist)
		if len(masterList.Variants) > 0 {
			chunk := masterList.Variants[0].Chunklist
			if chunk != nil {
				model.Duration += chunk.TargetDuration
			} else {
				uu, _ := url.Parse(model.Url)
				updateTankPageDuration(uu.Scheme+"://"+uu.Host+masterList.Variants[0].URI, model)
			}
		}
	}
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

		if record.Duration > 0 {
			track.Length = int(record.Duration)
		}

		var tags []m3u.Tag
		if len(record.Tags) > 0 {
			arr := strings.Split(record.Tags, ",")
			for _, s := range arr {
				tags = append(tags, m3u.Tag{
					Name:  "分类",
					Value: s,
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
	cmd := &cobra.Command{
		Use:   "tank",
		Short: "坦克资源网资源爬取",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			log.SetPrefix("Tank ")
			err := dao.DB.AutoMigrate(&TankModel{})
			if err != nil {
				log.Panicf("自动迁移失败: %v", err)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.WithValue(cmd.Context(), concurrentKey, *cnt)
			err := getTankPageLinks(ctx, *page)
			if err != nil {
				log.Fatalf("解析失败 : %v", err)
			}
		},
	}

	output := ""
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "导出为 m3u 格式",
		Run: func(cmd *cobra.Command, args []string) {
			exportTankPagesList(output)
		},
	}

	exportCmd.Flags().StringVarP(&output, "output", "o", "", "导出路径")
	_ = exportCmd.MarkFlagRequired("output")
	cnt = cmd.Flags().IntP(string(concurrentKey), "c", 10, "指定并发数量")
	page = cmd.Flags().IntP("page", "p", 1, "指定起始页码")
	cmd.AddCommand(exportCmd)

	token, debug := "", new(bool)
	upload := &cobra.Command{
		Use:   "upload",
		Short: "上传至 telegram 机器人频道",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.WithValue(cmd.Context(), telegram.Token, token)
			ctx = context.WithValue(ctx, telegram.Debug, *debug)

			f, err := os.CreateTemp("", "tank.m3u")
			if err != nil {
				log.Fatal(err)
			}
			_ = f.Close()
			defer os.Remove(f.Name())
			exportTankPagesList(f.Name())
			telegram.UploadToChannel(ctx, f.Name())
		},
	}

	upload.Flags().StringVarP(&token, "token", "t", "", "telegram bot 令牌")
	debug = upload.PersistentFlags().Bool("debug", false, "debug 模式")
	cmd.AddCommand(upload)

	return cmd
}
