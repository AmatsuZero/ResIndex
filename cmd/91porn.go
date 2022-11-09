package cmd

import (
	"ResIndex/dao"
	"ResIndex/telegram"
	"ResIndex/utils"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/jamesnetherton/m3u"
	"github.com/spf13/cobra"
)

type NinetyOneVideo struct {
	dao.M3U8Resource
	Author, Duration string
	AddedAt          time.Time
}

func extract91Links(ctx context.Context, htmlContent string, hasNextPage *bool) error {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return err
	}

	rows := document.Find("#wrapper > div.container.container-minheight > div.row > div > div").Children()
	models := make([]*NinetyOneVideo, 0, rows.Length())
	rows.Each(func(i int, selection *goquery.Selection) {
		model := &NinetyOneVideo{}
		sel := selection.Find(".well.well-sm.videos-text-align")
		a := sel.Find("a")
		ref, ok := a.Attr("href")
		if ok {
			model.Ref = sql.NullString{String: ref, Valid: true}
		} else {
			return
		}

		donCreate := !dao.NotExist(model, "ref = ?", ref) && len(model.Url) > 0
		title := a.Find(".video-title.title-truncate.m-t-5").Text()
		model.Name = sql.NullString{String: title, Valid: true}

		duration := a.Find(".duration").Text()
		model.Duration = duration

		thumbnail, ok := a.Find(".thumb-overlay").Find(".img-responsive").Attr("src")
		if ok {
			model.Thumbnail = sql.NullString{String: thumbnail, Valid: true}
		}

		// 查找作者
		html := selection.Text()
		lb := strings.Index(html, "作者:")
		rb := strings.Index(html, "热度:")
		name := strings.TrimSpace(html[lb:rb])
		parts := strings.Split(name, ":")
		name = strings.TrimSpace(parts[1])
		model.Author = name
		if !donCreate {
			dao.Create(model)
		}
		models = append(models, model)
	})

	*hasNextPage = false
	document.Find("#paging > div > form").Children().EachWithBreak(func(i int, selection *goquery.Selection) bool {
		txt := selection.Text()
		if txt == "»" {
			*hasNextPage = true
			return false
		}
		return true
	})

	return update91PornDetails(ctx, models)
}

func update91PornDetails(ctx context.Context, models []*NinetyOneVideo) error {
	concurrent := ctx.Value(concurrentKey).(int)
	ch := make(chan struct{}, concurrent)
	wg := &sync.WaitGroup{}
	const form = "2006-01-02"
	loc, _ := time.LoadLocation("Asia/Shanghai")

	for _, m := range models {
		wg.Add(1)
		ch <- struct{}{}

		go func(model *NinetyOneVideo) {
			defer wg.Done()

			if !model.Ref.Valid {
				return
			}

			html, err := utils.VisitWebPage(model.Ref.String)
			<-ch

			if err != nil {
				fmt.Printf("访问 %v 页面详情失败\n", model.Ref.String)
				return
			}

			document, err := goquery.NewDocumentFromReader(strings.NewReader(html))
			if err != nil {
				fmt.Printf("提取 %v 页面详情失败\n", model.Ref.String)
				return
			}
			src, ok := document.Find("#player_one_html5_api > source").Attr("src")
			if ok {
				model.Url = src
			}

			// 提取时间
			t := document.Find("#videodetails-content > div:nth-child(1) > span.title-yakov").Text()
			tt, e := time.ParseInLocation(form, t, loc)
			if e == nil {
				model.AddedAt = tt
			}
		}(m)
	}
	wg.Wait()
	// 保存一下
	dao.DB.Save(&models)
	return nil
}

func get91pornPageLinks(ctx context.Context, page int) error {
	hasNextPage := true
	for i := page; hasNextPage; i++ {
		url := fmt.Sprintf("https://91porn.com/v.php?category=rf&viewtype=basic&page=%v", i)
		log.Printf("开始提取第%v页内容", i)
		html, err := utils.VisitWebPage(url)
		if err != nil {
			log.Printf("访问第 %v 页失败\n", i)
			continue
		}
		err = extract91Links(ctx, html, &hasNextPage)
		if err != nil {
			log.Printf("提取第%v页链接失败, 继续下一页\n", i)
		} else {
			log.Printf("提取第%v页成功，即将开始下一页\n", i)
		}
	}

	return nil
}

func parseDuration(st string) (int, error) {
	var m, s int
	_, err := fmt.Sscanf(st, "%d:%d", &m, &s)
	return m*60 + s, err
}

func export91PageLinks(output string) {
	var records []*NinetyOneVideo
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
		if len(record.Tags) > 0 {
			arr := strings.Split(record.Tags, ",")
			for _, s := range arr {
				tags = append(tags, m3u.Tag{
					Name:  "分类",
					Value: s,
				})
			}
		}
		if len(record.Author) > 0 {
			tags = append(tags, m3u.Tag{
				Name:  "作者",
				Value: record.Author,
			})
		}
		dur, err := parseDuration(record.Duration)
		if err == nil {
			track.Length = dur
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

func download91Resources(exe, output string, concurrent int) {
	var records []*NinetyOneVideo
	dao.DB.Find(&records)

	ch := make(chan struct{}, concurrent)
	wg := &sync.WaitGroup{}

	for _, record := range records {
		wg.Add(1)
		ch <- struct{}{}

		go func(r *NinetyOneVideo) {
			defer wg.Done()
			msg, err := r.Download(exe, output)
			if err == nil {
				log.Printf("下载成功: %v", msg)
			} else {
				log.Printf("下载失败: %v", err)
			}
			<-ch
		}(record)
	}

	wg.Wait()
}

func NinetyOne() *cobra.Command {
	page, concurrent := new(int), new(int)

	cmd := &cobra.Command{
		Use:   "91",
		Short: "91 porn 资源爬取",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			log.SetPrefix("91porn ")
			err := dao.DB.AutoMigrate(&NinetyOneVideo{})
			if err != nil {
				log.Panicf("自动迁移失败: %v", err)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.WithValue(cmd.Context(), concurrentKey, *concurrent)
			log.SetPrefix("91porn ")
			err := get91pornPageLinks(ctx, *page)
			if err != nil {
				log.Fatalf("爬取失败：%v", err)
			} else {
				log.Printf("提取结束")
			}
		},
	}

	output := ""
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "导出为 m3u 格式",
		Run: func(cmd *cobra.Command, args []string) {
			export91PageLinks(output)
		},
	}

	downloadDir, downloadCurrent, execPath := "", new(int), ""
	downloadCmd := &cobra.Command{
		Use:   "download",
		Short: "下载资源",
		Run: func(cmd *cobra.Command, args []string) {
			err := utils.MakeDirSafely(downloadDir)
			if err != nil {
				log.Fatalf("创建文件夹失败: %v", err)
			}
			download91Resources(execPath, downloadDir, *downloadCurrent)
		},
	}

	exportCmd.Flags().StringVarP(&output, "output", "o", "", "导出路径")
	_ = exportCmd.MarkFlagRequired("output")

	page = cmd.Flags().IntP("page", "p", 1, "指定起始页码")
	concurrent = cmd.Flags().IntP("concurrent", "c", 5, "指定并发数量")

	downloadCmd.Flags().StringVarP(&downloadDir, "dir", "d", "", "下载文件夹")
	_ = downloadCmd.MarkFlagRequired("dir")
	downloadCurrent = downloadCmd.Flags().IntP("concurrent", "c", 3, "下载并发数")
	downloadCmd.Flags().StringVarP(&execPath, "exec", "e", "m3u8-downloader", "指定下载器路径")

	cmd.AddCommand(exportCmd)
	cmd.AddCommand(downloadCmd)

	token, debug := "", new(bool)
	upload := &cobra.Command{
		Use:   "upload",
		Short: "上传至 telegram 机器人频道",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.WithValue(cmd.Context(), telegram.Token, token)
			ctx = context.WithValue(ctx, telegram.Debug, *debug)
			ctx = context.WithValue(ctx, telegram.DownloaderPath, execPath)
			ctx = context.WithValue(ctx, telegram.ModelType, "91")
			dir, err := os.MkdirTemp("", "download")
			if err != nil {
				log.Fatalln(err)
			}
			defer os.RemoveAll(dir)
			telegram.UploadToChannel(ctx, dir)
		},
	}

	upload.Flags().StringVarP(&token, "token", "t", "", "telegram bot 令牌")
	debug = upload.PersistentFlags().Bool("debug", false, "debug 模式")
	upload.Flags().StringVarP(&execPath, "exec", "e", "m3u8-downloader", "指定下载器路径")
	cmd.AddCommand(upload)

	return cmd
}
