package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"

	"github.com/twinj/uuid"
)

const scratchDirectory string = "./scratch"

// Uploads holds a slice of Files uploaded during this session
type Uploads struct {
	Files []File
}

// File refers to one uploaded file
type File struct {
	Name         string
	OriginalName string
}

func deleteImage(w http.ResponseWriter, r *http.Request) {

	df := r.PostFormValue("df")

	uploads := getUploadsFromSession(w, r)

	for i, file := range uploads.Files {
		if isSafeName(file.OriginalName) {
			if file.OriginalName == df {
				log.Println("Delete file --->", file.Name)
				uploads.Files = append(uploads.Files[:i], uploads.Files[i+1:]...)
				os.Remove(scratchDirectory + "/" + file.Name)
				err := saveUploadsToSession(w, r, uploads)
				if err != nil {
					log.Println("Could not save uploaded file to session:", err)
				}
				log.Printf("Uploads: %+v\n", uploads)
			}
		} else {
			log.Println("Dangerous file name in session detected:", file.OriginalName)
		}
	}
}

func uploadImage(w http.ResponseWriter, r *http.Request) {

	var uploads *Uploads
	uploads = getUploadsFromSession(w, r)

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	file, info, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error retrieving file from upload request:", err)
		return
	}

	originalName := info.Filename

	defer file.Close()

	// Read the first 512 bytes of the image file so that we can detect the file format
	imgHeader := make([]byte, 512)
	n, err := io.ReadFull(file, imgHeader)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// If we can't even read 8 bytes from the file, it's definitely not a valid image
	// (8 bytes being the size of the PNG signature...)
	if n < 8 {
		log.Println("Error: invalid image format detected")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Detect the image format using the first 512 bytes of the image
	contentType := http.DetectContentType(imgHeader)

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

	err = saveUploadsToSession(w, r, uploads)
	if err != nil {
		log.Println("Could not save uploaded file to session:", err)

		// DELETE UPLOADED FILE AND NOTIFY CLIENT
	}

	log.Printf("Uploads: %+v", uploads)

	f, err := os.OpenFile(scratchDirectory+"/"+uuidFilename, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer f.Close()

	written, _ := io.Copy(f, file)

	result := fmt.Sprintf("Wrote %v bytes to %v", written, uuidFilename)

	w.Write([]byte(result))
}

func getUploadsFromSession(w http.ResponseWriter, r *http.Request) *Uploads {
	var uploads Uploads
	var ok bool
	var uploadsEncoded string

	gob.Register(&Uploads{})

	sess := getSession(w, r)
	if sess.IsNew {
		return &uploads
	}

	if uploadsEncoded, ok = sess.Values["uploads"].(string); ok {
		buf := bytes.NewBufferString(uploadsEncoded)
		dec := gob.NewDecoder(buf)
		dec.Decode(&uploads)
	}
	return &uploads

}

func saveUploadsToSession(w http.ResponseWriter, r *http.Request, u *Uploads) error {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	enc.Encode(u)

	sess := getSession(w, r)
	sess.Values["uploads"] = b.String()

	err := saveSession(w, r, sess)

	return err
}

func isSafeName(name string) bool {

	astMatch, _ := regexp.MatchString(".*\\*.*", name)
	if astMatch {
		return false
	}

	dir, base := path.Split(name)
	if dir != "" {
		return false
	}
	if base == "" {
		return false
	}
	switch base[0] {
	case '.', '-':
		return false
	}
	return true
}
