package cmd

import (
	"ResIndex/cmd/sis001"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"log"
	"time"
)

func Sis() *cobra.Command {
	cnt := new(int)

	root := &cobra.Command{
		Use:   "sis",
		Short: "sis001 下载",
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			ch := make(chan struct{}, *cnt)
			ctx := context.WithValue(cmd.Context(), sis001.ConcurrentKey, ch)
			cmd.SetContext(ctx)
			sis001.PreRun(cmd)
		},
		PersistentPostRun: func(cmd *cobra.Command, _ []string) {
			t, ok := cmd.Context().Value(sis001.StartTimeKey).(time.Time)
			if !ok {
				return
			}
			now := time.Now()
			log.Printf("🚀 任务结束，耗时：%v\n", now.Sub(t))
		},
	}

	root.AddCommand(sis001.NewArticle())
	cnt = root.PersistentFlags().IntP("concurrent", "c", 3, "指定并发数")

	return root
}
