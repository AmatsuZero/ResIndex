package cmd

import (
	"ResIndex/dao"
	"ResIndex/utils"
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
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

		donCreate := !dao.Any(model, "ref = ?", ref) && len(model.Url) > 0
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

			html, err := Visit91Page(model.Ref.String)
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

func Visit91Page(url string) (html string, err error) {
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
		"91porn.com",
		map[string]interface{}{
			"Accept-Language": "zh-cn,zh;q=0.5",
			"X-Forwarded-For": genIpaddr(),
		},
		&res,
	))

	timeoutCtx, cancel := context.WithTimeout(chromeCtx, 1000*time.Second)
	defer cancel()

	acts := []chromedp.Action{
		chromedp.Navigate(url),
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

	err = chromedp.Run(timeoutCtx, acts...)

	if err != nil {
		log.Printf("Run err : %v\n\n", err)
		_ = chromedp.Cancel(timeoutCtx)
		return
	}
	return
}

func get91pornPageLinks(ctx context.Context, page int) error {
	hasNextPage := true
	for i := page; hasNextPage; i++ {
		url := fmt.Sprintf("https://91porn.com/v.php?category=rf&viewtype=basic&page=%v", i)
		log.Printf("开始提取第%v页内容", i)
		html, err := Visit91Page(url)
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
			r.Download(exe, output)
			<-ch
		}(record)
	}

	wg.Wait()
}

func NinetyOne() *cobra.Command {
	migrate := func(cmd *cobra.Command, args []string) {
		err := dao.DB.AutoMigrate(&NinetyOneVideo{})
		if err != nil {
			log.Panicf("自动迁移失败: %v", err)
		}
	}

	page, concurrent := new(int), new(int)

	ctx := context.Background()
	cmd := &cobra.Command{
		Use:    "91",
		Short:  "91 porn 资源爬取",
		PreRun: migrate,
		Run: func(cmd *cobra.Command, args []string) {
			ctx = context.WithValue(ctx, concurrentKey, *concurrent)
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
		Use:    "export",
		Short:  "导出为 m3u 格式",
		PreRun: migrate,
		Run: func(cmd *cobra.Command, args []string) {
			export91PageLinks(output)
		},
	}

	downloadDir, downloadCurrent, execPath := "", new(int), ""
	downloadCmd := &cobra.Command{
		Use:    "download",
		Short:  "下载资源",
		PreRun: migrate,
		Run: func(cmd *cobra.Command, args []string) {
			log.SetPrefix("91porn ")
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

	return cmd
}
