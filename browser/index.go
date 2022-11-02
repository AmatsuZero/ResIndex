package browser

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"log"
	"net/http"
)

func Cmd() *cobra.Command {
	port := new(int)
	browser := &cobra.Command{
		Use:   "browse",
		Short: "浏览本地数据",
		Run: func(cmd *cobra.Command, args []string) {
			r := mux.NewRouter()
			fs := http.FileServer(http.Dir("browser/dist"))
			r.PathPrefix("/").Handler(fs)
			http.Handle("/", r)
			log.Println("Listening")
			startServer()
			log.Panic(http.ListenAndServe(fmt.Sprintf(":%v", port), nil))
		},
	}

	port = browser.Flags().IntP("port", "p", 8080, "指定端口号")
	return browser
}

func startServer() {

}
