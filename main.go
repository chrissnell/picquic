package main

import (
	"log"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gorilla/sessions"
	"github.com/yosssi/boltstore/reaper"
)

const maxUploadSize int64 = 20 * 1024 * 1024

var store *sessions.CookieStore

func indexPage(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/" {
		http.ServeFile(w, r, "./assets/"+r.URL.Path)
		return
	}

	// Page has been reloaded; clear out any old sessions
	log.Println("Clearing out old sessions...")
	sess := getSession(w, r)
	sess.Options.MaxAge = -1
	saveSession(w, r, sess)

	// Serve up our index.html
	http.ServeFile(w, r, "assets/index.html")
}

func getSession(w http.ResponseWriter, r *http.Request) *sessions.Session {
	session, err := store.Get(r, "picquic")
	if err != nil {
		log.Println("Error fetching session:", err)
		log.Println("Attempting to reset session by creating and saving new session")
		err = session.Save(r, w)
		if err != nil {
			log.Println("Unable to reset session by creating and saving new session:", err)
		}
	}
	return session
}

func saveSession(w http.ResponseWriter, r *http.Request, ses *sessions.Session) error {
	err := sessions.Save(r, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	return nil
}

func main() {

	store = sessions.NewCookieStore([]byte("picquic-secretz"))
	store.MaxAge(60 * 10)

	db, err := bolt.Open("./db/sessions.db", 0666, nil)
	if err != nil {
		log.Fatalln("Could not open sessions DB:", err)
	}
	defer db.Close()

	// Invoke a reaper which checks and removes expired sessions periodically.
	defer reaper.Quit(reaper.Run(db, reaper.Options{}))

	http.HandleFunc("/", indexPage)

	http.Handle("/css", http.StripPrefix("/css", http.FileServer(http.Dir("./assets/css"))))
	http.Handle("/js", http.StripPrefix("/js", http.FileServer(http.Dir("./assets/js"))))
	http.Handle("/fonts", http.StripPrefix("/fonts", http.FileServer(http.Dir("./assets/fonts"))))

	http.HandleFunc("/upload", uploadImage)

	http.HandleFunc("/delete", deleteImage)

	http.ListenAndServe(":9000", nil)
}
