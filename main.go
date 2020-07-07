package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"

)


func main() {

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w,"/welcome")
	})

	log.Println("running!")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
