package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

const maxUploadSize int64 = 10 * 1024 * 1024

func limitedUpload(rw http.ResponseWriter, req *http.Request) {
	req.ParseMultipartForm(maxUploadSize)
	file, handler, err := req.FormFile("file")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()
	log.Println("handler.Header:", handler.Header)

	f, err := os.OpenFile("./scratch/"+handler.Filename, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	written, _ := io.Copy(f, file)
        fmt.Fprintf(rw, "Wrote %v bytes to %v", written, handler.Filename)


}

func main() {
	fs := http.FileServer(http.Dir("./assets"))
	http.Handle("/", http.StripPrefix("/", fs))
	http.HandleFunc("/upload", limitedUpload)
	log.Fatal(http.ListenAndServe(":9000", nil))
}
