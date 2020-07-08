package main

import (
	"fmt"
	"kube/agent"
	"log"
	"net/http"
	_ "net/http/pprof"
)


func main() {

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w,"/welcome")
	})

	go agent.StartAgent()
	simulate()
	log.Println("running!")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
