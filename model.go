package main

import (
	"database/sql"
	"errors"
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"path/filepath"
	"time"
)

type M3U8Resource struct {
	gorm.Model
	Name      sql.NullString
	Url       string `gorm:"uniqueIndex"`
	Thumbnail sql.NullString
	Tags      []sql.NullString `gorm:"type:string[]"`
	Ref       sql.NullString
}

var DB *gorm.DB

func init() {
	log.SetPrefix("DB ")
	appFolder, _ := os.UserConfigDir()
	if len(appFolder) == 0 {
		appFolder = os.Getenv("APPDATA")
	}
	appFolder = filepath.Join(appFolder, "M3U8-Downloader-GO")
	if _, err := os.Stat(appFolder); errors.Is(err, os.ErrNotExist) {
		err = os.Mkdir(appFolder, os.ModePerm)
		if err != nil {
			return
		}
	}

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer（日志输出的目标，前缀和日志包含的内容——译者注）
		logger.Config{
			SlowThreshold:             time.Second,   // 慢 SQL 阈值
			LogLevel:                  logger.Silent, // 日志级别
			IgnoreRecordNotFoundError: true,          // 忽略ErrRecordNotFound（记录未找到）错误
			Colorful:                  false,         // 禁用彩色打印
		},
	)

	dbPath := filepath.Join(appFolder, "tank.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		log.Fatalln("failed to connect database")
	}
	DB = db

	err = db.AutoMigrate(&M3U8Resource{})
	if err != nil {
		log.Panicf("自动迁移失败: %v", err)
	}
}

func (r *M3U8Resource) Any(conds ...interface{}) bool {
	result := DB.First(r, conds...)
	return errors.Is(result.Error, gorm.ErrRecordNotFound)
}

func (r *M3U8Resource) Create(conds ...interface{}) {
	DB.FirstOrCreate(r, conds...)
}

func (r *M3U8Resource) Save() {
	DB.Save(r)
}

func (r *M3U8Resource) String() string {
	return fmt.Sprintf("link： %v, name: %v tag: %v", r.Url, r.Name, r.Tags)
}