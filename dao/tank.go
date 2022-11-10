package dao

import (
	"fmt"
	"time"
)

type TankModel struct {
	M3U8Resource
	UpdateTime time.Time
	Duration   float64
}

func (v *TankModel) MarkDownRender() string {
	return fmt.Sprintf(`
标题：%v
时长：%v
预览图：[图片](%v)
视频链接：[点我查看](%v)`, v.Name.String, floatToDuration(int64(v.Duration)), v.Thumbnail.String,
		v.Url)
}

func floatToDuration(num int64) string {
	hour := num / 3600
	minute := (num / 60) % 60
	second := num % 60
	return fmt.Sprintf("%d:%02d:%02d\n", hour, minute, second)
}
