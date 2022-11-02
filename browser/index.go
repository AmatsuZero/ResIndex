package browser

import (
	"ResIndex/cmd"
	"ResIndex/dao"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"log"
	"net/http"
	"strconv"
)

func Cmd() *cobra.Command {
	port := new(int)

	browser := &cobra.Command{
		Use:   "browse",
		Short: "浏览本地数据",
		Run: func(cmd *cobra.Command, args []string) {
			r := gin.Default()
			r.LoadHTMLGlob("browser/templates/*")
			startServer(r, *port)
		},
	}

	port = browser.Flags().IntP("port", "p", 8080, "指定端口号")

	return browser
}

func startServer(r *gin.Engine, port int) {
	r.GET("/91", ninetyOne)
	r.GET("/tank", tank)

	err := r.Run(fmt.Sprintf(":%v", port))
	if err != nil {
		log.Fatalln(err)
	}
}

func ninetyOne(c *gin.Context) {
	err := dao.DB.AutoMigrate(&cmd.NinetyOneVideo{})
	if err != nil {
		c.Error(err)
	}

	id, arg := 0, c.Query("prev")
	if len(arg) == 0 {
		arg = c.Query("next")
	} else {
		num, _ := strconv.Atoi(arg)
		id = num - 1
	}

	if len(arg) == 0 {
		id = 1
	} else {
		num, _ := strconv.Atoi(arg)
		id = num + 1
	}

	video := &cmd.NinetyOneVideo{}
	dao.DB.First(&video, "id = ?", id)

	c.HTML(http.StatusOK, "video.tmpl", gin.H{
		"title": video.Name.String,
		"img":   video.Thumbnail.String,
		"id":    video.ID,
	})
}

func tank(c *gin.Context) {

}
