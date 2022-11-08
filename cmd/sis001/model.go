package sis001

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"gorm.io/gorm"
	"path"
	"regexp"
	"strings"
)

type InfoModel struct {
	gorm.Model
	ThreadId                                string
	Title, Format, PostId, Sig, TorrentLink string
	Category, Tag, Size                     string
	IsPosted, IsBlurred                     bool
	Actors, Thumbnails                      string
}

func getSplitValue(str string) string {
	separator := "："
	idx := strings.LastIndex(str, separator)
	if idx != -1 {
		str = str[idx+1:]
	}
	return strings.TrimSpace(str)
}

func isBlurred(str string) bool {
	str = getSplitValue(str)
	marks := []string{"有码", "有碼", "薄码"}
	for _, mark := range marks {
		if mark == str {
			return true
		}
	}
	return false
}

func getActors(str string) string {
	val := strings.Split(getSplitValue(str), " ")
	re := regexp.MustCompile(`/[^\p{L}\p{N}\p{Z}]/gu`)
	tmp := val[0:]
	for _, v := range val {
		if v == "等" {
			continue
		}
		v = re.ReplaceAllString(v, "")
		if len(v) > 0 {
			tmp = append(tmp, v)
		}
	}
	return strings.Join(tmp, ",")
}

func (i *InfoModel) FillInfo(lines []string) {
	for _, line := range lines {
		switch {
		case strings.Contains(line, "影片名稱"):
			i.Title = getSplitValue(line)
		case strings.Contains(line, "影片格式"):
			i.Format = getSplitValue(line)
		case strings.Contains(line, "影片大小"),
			strings.Contains(line, "视频大小"):
			i.Size = getSplitValue(line)
		case strings.Contains(line, "是否有碼"):
			i.IsBlurred = isBlurred(line)
		case strings.Contains(line, "种子特征码"),
			strings.Contains(line, "特徵碼"):
			i.Sig = getSplitValue(line)
		case strings.Contains(line, "出演女優"):
			i.Actors = getActors(line)
		}
	}
}

func (i *InfoModel) ExtractNewListModelTitle(doc *goquery.Document) {
	sel := "#pid" + i.PostId + " > tbody > tr:nth-child(1) > td.postcontent > div.postmessage.defaultpost > h2"
	i.Title = doc.Find(sel).Text()
}

func (i *InfoModel) ExtractNewListModelThumbnail(doc *goquery.Document) {
	sel := "#postmessage_" + i.PostId + " > img"
	links := doc.Find(sel).Map(func(i int, selection *goquery.Selection) string {
		src, _ := selection.Attr("src")
		return src
	})
	for _, link := range links {
		if len(link) == 0 {
			continue
		}
		if path.Ext(link) != ".gif" { // gif 图片是宣传图片，需要过滤掉
			i.Thumbnails += fmt.Sprintf(",%v", link)
		}
	}
}
