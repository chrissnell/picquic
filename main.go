package main

import (
	"log"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/yosssi/boltstore/reaper"
)

const maxUploadSize int64 = 20 * 1024 * 1024

var store = sessions.NewCookieStore(securecookie.GenerateRandomKey(16))

func getSession(w http.ResponseWriter, r *http.Request) *sessions.Session {
	session, err := store.Get(r, "picquic")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error fetching session:", err)
		return nil
	}
	return session
}

func saveSession(w http.ResponseWriter, r *http.Request, ses *sessions.Session) {
	err := sessions.Save(r, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error saving session:", err)
		return
	}
}

func main() {

	db, err := bolt.Open("./sessions.db", 0666, nil)
	if err != nil {
		log.Fatalln("Could not open sessions DB:", err)
	}
	defer db.Close()

	// Invoke a reaper which checks and removes expired sessions periodically.
	defer reaper.Quit(reaper.Run(db, reaper.Options{}))

	http.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir("./assets"))))

	http.HandleFunc("/upload", uploadImage)

	http.HandleFunc("/delete", deleteImage)

	http.ListenAndServe(":9000", nil)
}
