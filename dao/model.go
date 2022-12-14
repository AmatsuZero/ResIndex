package dao

import (
	"ResIndex/utils"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type M3U8Resource struct {
	gorm.Model
	Name      sql.NullString
	Url       string `gorm:"unique"`
	Thumbnail sql.NullString
	Tags      string
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
	err := utils.MakeDirSafely(appFolder)
	if err != nil {
		log.Fatalf("创建文件夹失败，错误：%v", err)
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

func NotExist(r interface{}, conds ...interface{}) bool {
	result := DB.First(r, conds...)
	return errors.Is(result.Error, gorm.ErrRecordNotFound)
}

func Create(r interface{}) {
	DB.Create(r)
}

func (r *M3U8Resource) String() string {
	return fmt.Sprintf("link： %v, name: %v tag: %v", r.Url, r.Name, r.Tags)
}

func (r *M3U8Resource) Download(exe, output string) (string, error) {
	args := []string{
		"--headless",
		"--downloadDir", output,
		"parse", "-u", r.Url,
	}

	if r.Name.Valid {
		n := r.Name.String
		p := filepath.Join(output, n)
		if utils.IsPathExist(p) { // 检查文件是不是已经有同名文件了
			n += fmt.Sprintf("(%v)", r.ID)
		}
		args = append(args, "-n", n)
	}

	msg, err := utils.Cmd(exe, args)
	return msg, err
}
