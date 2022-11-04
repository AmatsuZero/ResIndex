package browser

import (
	"ResIndex/cmd"
	"ResIndex/dao"
	"encoding/json"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

// AddRouterEndpoints add the actual endpoints for api
func AddRouterEndpoints(r *mux.Router) *mux.Router {
	r.HandleFunc("/91", get91Videos).Methods("GET")
	return r
}

func sendJSONResponse(w http.ResponseWriter, data interface{}) {
	body, err := json.Marshal(data)
	if err != nil {
		log.Printf("Failed to encode a JSON response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(body)
	if err != nil {
		log.Printf("Failed to write the response body: %v", err)
		return
	}
}

func get91Videos(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page := query.Get("page")
	if len(page) == 0 {
		page = "1"
	}

	var videos []*cmd.NinetyOneVideo
	dao.DB.Take(&videos)
	sendJSONResponse(w, videos)
}
