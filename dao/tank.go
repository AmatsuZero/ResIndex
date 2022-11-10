package dao

import "time"

type TankModel struct {
	M3U8Resource
	UpdateTime time.Time
	Duration   float64
}
