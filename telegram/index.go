package telegram

import (
	"ResIndex/dao"
	"context"
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

type CtxKey string

const (
	Token          CtxKey = "token"
	Debug          CtxKey = "debug"
	DownloaderPath CtxKey = "downloader"
)

func Cmd() *cobra.Command {
	token, debug, execPath := "", new(bool), ""
	cmd := &cobra.Command{
		Use:   "bot",
		Short: "接通 telegram 机器人",
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			log.SetPrefix("🤖 ")
			ctx := context.WithValue(cmd.Context(), Token, token)
			ctx = context.WithValue(ctx, Debug, *debug)
			ctx = context.WithValue(ctx, DownloaderPath, execPath)

			cmd.SetContext(ctx)
		},
		Run: startBot,
	}

	cmd.Flags().StringVarP(&token, "token", "t", "", "telegram bot 令牌")
	_ = cmd.MarkFlagRequired("token")

	debug = cmd.Flags().Bool("debug", false, "debug 模式")
	cmd.Flags().StringVarP(&execPath, "exec", "e", "m3u8-downloader", "指定下载器路径")

	return cmd
}

func startBot(cmd *cobra.Command, _ []string) {
	ctx := cmd.Context()
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
	log.Println("telegram bot 已经启动")

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := bot.GetUpdatesChan(updateConfig)
	ch := make(chan struct{}, 10)

	for u := range updates {
		ch <- struct{}{}

		go func(update tgbotapi.Update) {
			if update.Message == nil {
				return
			}

			tmpDir, e := os.MkdirTemp("", fmt.Sprintf("bot-%v", update.Message.MessageID))
			if e != nil {
				return
			}

			defer func() {
				<-ch
				_ = os.RemoveAll(tmpDir)
			}()

			var msg tgbotapi.Chattable
			if !update.Message.IsCommand() {
				return
			}

			switch update.Message.Command() {
			case "91":
				msg = replyTo91Message(ctx, update, tmpDir)
			case "tank":
				msg = replyToTankMessage(ctx, update, tmpDir)
			default:
				msg = createReplyTxt(update, update.Message.Text)
			}

			if msg == nil {
				return
			}

			// Okay, we're sending our message off! We don't care about the message
			// we just sent, so we'll discard it.
			if _, err = bot.Send(msg); err != nil {
				log.Printf("send message err: %v\n", err)
			}
		}(u)
	}
}

func createReplyTxt(update tgbotapi.Update, txt string) tgbotapi.MessageConfig {
	textMsg := tgbotapi.NewMessage(update.Message.Chat.ID, txt)
	textMsg.ReplyToMessageID = update.Message.MessageID
	return textMsg
}

func replyToTankMessage(ctx context.Context, update tgbotapi.Update, dir string) tgbotapi.Chattable {
	items, err := url.ParseQuery(update.Message.CommandArguments())
	if err != nil {
		return nil
	}

	id := items.Get("id") // 获取
	model := &dao.TankModel{}
	num, _ := strconv.Atoi(id)
	model.ID = uint(num)
	db := dao.DB.First(&model)

	if errors.Is(db.Error, gorm.ErrRecordNotFound) {
		return createReplyTxt(update, "没有找到")
	}

	switch {
	case items.Has("preview"):
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		msg.ParseMode = tgbotapi.ModeMarkdown
		msg.Text = model.MarkDownRender()
		return msg

	case items.Has("video"):
		exe, ok := ctx.Value(DownloaderPath).(string)
		if !ok || len(exe) == 0 {
			return createReplyTxt(update, "无法下载")
		}

		// 下载
		_, e := model.Download(exe, dir)
		if e != nil {
			return createReplyTxt(update, fmt.Sprintf("视频下载失败： %v", e))
		}

		files, e := os.ReadDir(dir)
		if e != nil || len(files) == 0 {
			return createReplyTxt(update, fmt.Sprintf("视频下载失败： %v", e))
		}

		for _, file := range files {
			if filepath.Ext(file.Name()) == ".mp4" { // 找到最近的文件
				video := tgbotapi.FilePath(filepath.Join(dir, file.Name()))
				return tgbotapi.NewVideo(update.Message.Chat.ID, video)
			}
		}

		return createReplyTxt(update, "下载失败")

	default:
		return createReplyTxt(update, "不知道应该怎么响应")
	}
}

func replyTo91Message(ctx context.Context, update tgbotapi.Update, dir string) tgbotapi.Chattable {
	items, err := url.ParseQuery(update.Message.CommandArguments())
	if err != nil {
		return nil
	}

	id := items.Get("id") // 获取
	model := &dao.NinetyOneVideo{}
	num, _ := strconv.Atoi(id)
	model.ID = uint(num)
	db := dao.DB.First(&model)

	if errors.Is(db.Error, gorm.ErrRecordNotFound) {
		return createReplyTxt(update, "没有找到")
	}

	switch {
	case items.Has("preview"):
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		msg.ParseMode = tgbotapi.ModeMarkdown
		msg.Text = model.MarkDownRender()
		return msg

	case items.Has("video"):
		exe, ok := ctx.Value(DownloaderPath).(string)
		if !ok || len(exe) == 0 {
			return createReplyTxt(update, "无法下载")
		}

		files, e := os.ReadDir(dir)
		if e != nil || len(files) == 0 {
			return createReplyTxt(update, fmt.Sprintf("视频下载失败： %v", e))
		}

		for _, file := range files {
			if filepath.Ext(file.Name()) == ".mp4" { // 找到最近的文件
				video := tgbotapi.FilePath(filepath.Join(dir, file.Name()))
				return tgbotapi.NewVideo(update.Message.Chat.ID, video)
			}
		}

		return createReplyTxt(update, "下载失败")

	default:
		return createReplyTxt(update, "不知道应该怎么响应")
	}
}
