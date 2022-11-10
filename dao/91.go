package dao

import (
	"fmt"
	"time"
)

type NinetyOneVideo struct {
	M3U8Resource
	Author, Duration string
	AddedAt          time.Time
}

func (v *NinetyOneVideo) MarkDownRender() string {
	return fmt.Sprintf(`
标题：%v
作者：%v
时长：%v
预览图：[图片](%v)
上传时间：%v
视频链接：[点我查看](%v)`, v.Name.String, v.Author, v.Duration,
		v.Thumbnail.String, v.AddedAt, v.Url)
}
