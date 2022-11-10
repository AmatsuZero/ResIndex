package dao

import "time"

type NinetyOneVideo struct {
	M3U8Resource
	Author, Duration string
	AddedAt          time.Time
}
