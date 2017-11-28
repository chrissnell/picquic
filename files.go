package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/kataras/iris"
	"github.com/twinj/uuid"
)

func deleteImage(ctx *Context) {
	uploads := getUploadsCookie(ctx)

	for _, file := range uploads.Files {
		log.Println("file:", file.Name)
	}

	log.Println(uploads)
}

func uploadImage(ctx *Context) {

	var uploads *Uploads
	uploads = getUploadsCookie(ctx)

	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		ctx.Application().Logger().Warnf("Error while uploading: %v", err.Error())
		return
	}

	originalName := info.Filename

	defer file.Close()

	// Read the first 512 bytes of the image file so that we can detect the file format
	imgHeader := make([]byte, 512)
	n, err := io.ReadFull(file, imgHeader)
	if err != nil {
		log.Println(err)
		ctx.StatusCode(500)
		return
	}
	// If we can't even read 8 bytes from the file, it's definitely not a valid image
	// (8 bytes being the size of the PNG signature...)
	if n < 8 {
		ctx.Values().Set("message", "Invalid image detected.")
		ctx.StatusCode(500)
		return
	}

	// Detect the image format using the first 512 bytes of the image
	contentType := http.DetectContentType(imgHeader)
	log.Println("Detected content type:", contentType)

	var fileExtension string
	switch contentType {
	case "image/jpeg":
		fileExtension = "jpg"
	case "image/gif":
		fileExtension = "gif"
	case "image/png":
		fileExtension = "png"
	case "image/webp":
		fileExtension = "webp"
	}

	// Seek back to the beginning of the file so that we can write
	// a complete file to disk
	file.Seek(0, io.SeekStart)

	//uuid.Init()
	uuid.SwitchFormat(uuid.FormatCanonical)
	uuidFilename := uuid.NewV4().String() + "." + fileExtension

	thisFile := File{
		Name:         uuidFilename,
		OriginalName: originalName,
	}

	uploads.Files = append(uploads.Files, thisFile)
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	enc.Encode(uploads)

	ctx.Session().Set("uploads", b.String())

	log.Printf("Uploads: %+v", uploads)

	f, err := os.OpenFile("./scratch/"+uuidFilename, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println(err)
		return
	}

	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		ctx.Application().Logger().Warnf("Error while preparing the new file: %v", err.Error())
		return
	}
	defer f.Close()

	written, _ := io.Copy(f, file)

	result := fmt.Sprintf("Wrote %v bytes to %v", written, uuidFilename)

	ctx.Text(result)

}

func getUploadsCookie(ctx *Context) *Uploads {
	var uploads Uploads

	gob.Register(&Uploads{})

	upsEncoded := ctx.Session().Get("uploads")

	if upsEncoded != nil {
		buf := bytes.NewBufferString(upsEncoded.(string))
		dec := gob.NewDecoder(buf)
		dec.Decode(&uploads)
	}

	return &uploads
}
