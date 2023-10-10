package main

import (
	"flag"
	"log"
	"net/http"
	"ws_chatroom/internal/entities"
)

func main() {

	port := flag.String("port", "3434", "set port")
	flag.Parse()
	hub := entities.NewHub()
	go hub.Run()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { //瀏覽器 http://localhost:3434/  會開啟index.html
		http.ServeFile(w, r, "index.html")
	})
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) { // client打websocket
		entities.WsServie(hub, w, r)
	})
	log.Println(":" + *port)
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Printf("ListenAndServe error:%s", err.Error())
		return
	}

}
