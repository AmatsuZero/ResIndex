package cmd

import (
	"ResIndex/utils"
	"fmt"
	"github.com/chromedp/chromedp"
	"github.com/spf13/cobra"
	"net/url"
	"time"
)

func Spankbang() *cobra.Command {
	u := ""
	cmd := &cobra.Command{
		Use:     "spank",
		Short:   "spankbang.com 视频链接下载",
		Aliases: []string{"sb"},
		Run: func(cmd *cobra.Command, args []string) {
			getDownloadLink(u)
		},
	}

	cmd.PersistentFlags().StringVarP(&u, "url", "u", "", "视频url 地址")
	return cmd
}

func getDownloadLink(u string) error {
	addr, err := url.Parse(u)
	if err != nil {
		return err
	}

	err = utils.VisitWebPageWithActions(
		fmt.Sprintf("%v://%v", addr.Scheme, addr.Host),
		chromedp.Navigate(addr.String()),
		chromedp.Sleep(100*time.Second),
		chromedp.WaitVisible("document.querySelector(\"#video > div.left > ul.video_toolbar > li.dl > svg > use\")", chromedp.ByJSPath),
		chromedp.Click("document.querySelector(\"#video > div.left > ul.video_toolbar > li.dl > svg > use\")", chromedp.ByJSPath),
	)

	if err != nil {
		return err
	}

	return nil
}
