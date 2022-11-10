package main

import (
	"ResIndex/browser"
	"ResIndex/cmd"
	"ResIndex/dao"
	"ResIndex/telegram"
	"ResIndex/utils"
	"github.com/spf13/cobra"
)

func initConfig() {
	dao.InitDB()
	utils.SetupWebClient()
}

func main() {
	cobra.OnInitialize(initConfig)

	rootCmd := &cobra.Command{
		Use: "index",
	}

	rootCmd.AddCommand(
		cmd.Tank(),
		cmd.NinetyOne(),
		cmd.EHentai(),
		cmd.Sis(),
		browser.Cmd(),
		telegram.Cmd(),
	)
	_ = rootCmd.Execute()
}
