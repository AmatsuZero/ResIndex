package dao

import (
	"ResIndex/utils"
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

func InitDB() {
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
}

func Any(r interface{}, conds ...interface{}) bool {
	result := DB.First(r, conds...)
	return errors.Is(result.Error, gorm.ErrRecordNotFound)
}

func Create(r interface{}) {
	DB.Create(r)
}

func (r *M3U8Resource) String() string {
	return fmt.Sprintf("link： %v, name: %v tag: %v", r.Url, r.Name, r.Tags)
}

func (r *M3U8Resource) Download(exe, output string) {
	args := []string{
		"--headless",
		"-downloadDir", output,
		"parse", "-u", r.Url,
	}

	if r.Name.Valid {
		args = append(args, "-n", r.Name.String)
	}

	msg, err := utils.Cmd(exe, args)
	if err != nil {
		log.Printf("下载成功: %v", msg)
	} else {
		log.Printf("下载失败: %v", err)
	}
}
