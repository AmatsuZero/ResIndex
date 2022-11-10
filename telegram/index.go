package telegram

import (
	"ResIndex/dao"
	"context"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"os"
	"path/filepath"
)

type CtxKey string

const (
	Token          CtxKey = "token"
	Debug          CtxKey = "debug"
	DownloaderPath CtxKey = "downloader"
	ModelType      CtxKey = "model-type"
)

func UploadToChannel(ctx context.Context, dir string) {
	debug, _ := ctx.Value(Debug).(bool)
	token, ok := ctx.Value(Token).(string)
	if !ok || len(token) == 0 {
		log.Fatalln("需要设置令牌")
	}

	exe, ok := ctx.Value(DownloaderPath).(string)
	if !ok || len(exe) == 0 {
		log.Fatalln("需要设置下载器地址")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = debug

	modelType, ok := ctx.Value(ModelType).(string)
	if !ok {
		return
	}
	switch modelType {
	case "91":
		upload91Model(exe, dir, bot)
	}
}

func upload91Model(exe, dir string, bot *tgbotapi.BotAPI) {
	var records []*dao.NinetyOneVideo
	dao.DB.Find(&records)

	for _, record := range records {
		// 下载
		msg, err := record.Download(exe, dir)
		if err == nil {
			log.Printf("下载成功: %v", msg)
		} else {
			log.Printf("下载失败: %v", err)
			continue
		}

		n := record.Name.String
		p := filepath.Join(dir, n)

		// 上传完毕，删除
		_ = os.Remove(p)
	}
}
