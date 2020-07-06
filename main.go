package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/pprof"
	"sync/atomic"
	"time"
)

var libprofile *pprof.Profile

func init() {
	fmt.Println("init called")
	profName := "my_experiment"
	libprofile = pprof.Lookup(profName)
	if libprofile == nil {
		libprofile = pprof.NewProfile(profName)
	}
}

type someResource struct {
	*os.File
}

var fileIndex = int64(0)

func MustResource() *someResource {
	f, err := os.Create(fmt.Sprintf("%d.txt", atomic.AddInt64(&fileIndex,1)))

	if err != nil {
		panic(err)
	}

	r:= &someResource{f}

	libprofile.Add(r,1)

	return r
}

func (r* someResource) Close() error {
	libprofile.Remove(r)
	return r.File.Close()
}

func trackAFunction() {
	tracked := new(byte)
	libprofile.Add(tracked,1)
	defer libprofile.Remove(tracked)
	time.Sleep(time.Second)
}
func usesAResource() {
	res := MustResource()
	defer res.Close()
	for i:=0;i<10;i++ {
		time.Sleep(time.Second)
	}
}

func main() {

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w,"/")
	})

	http.HandleFunc("/hi", func(w http.ResponseWriter, r *http.Request){
		fmt.Fprintf(w,"/hi")
	})
	http.HandleFunc("/nonblock", func(rw http.ResponseWriter, req *http.Request) {
		go usesAResource()
		fmt.Fprintf(rw,"/nonblock")
	})
	http.HandleFunc("/functiontrack", func(rw http.ResponseWriter, req *http.Request) {
		trackAFunction()
		fmt.Fprintf(rw,"/functiontrack")
	})
	http.HandleFunc("/block", func(rw http.ResponseWriter, req *http.Request) {
		usesAResource()
		fmt.Fprintf(rw,"/block")
	})
	log.Println("running!")

	log.Fatal(http.ListenAndServe(":8080", nil))

}