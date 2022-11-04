package main

import (
	"ResIndex/browser"
	"ResIndex/cmd"
	"ResIndex/dao"

	"github.com/spf13/cobra"
)

func initConfig() {
	dao.InitDB()
}

func main() {
	cobra.OnInitialize(initConfig)
	rootCmd := &cobra.Command{Use: "index"}
	rootCmd.AddCommand(
		cmd.Tank(),
		cmd.NinetyOne(),
		cmd.EHentai(),
		browser.Cmd(),
	)
	_ = rootCmd.Execute()
}
