package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/kataras/iris"
	"github.com/kataras/iris/sessions"
	"github.com/kataras/iris/sessions/sessiondb/boltdb"
)

const maxUploadSize int64 = 20 * 1024 * 1024

// Uploads holds the list of files uploaded during this session
type Uploads struct {
	Files []string
}

// Picquic is our application structure with the fields and methods
// that we need for the app.
type Picquic struct {
	sessionsManager *sessions.Sessions
}

// This package-level variable is used within the context to communicate
// with the greater application.
var pq = &Picquic{
	sessionsManager: sessions.New(sessions.Config{
		Cookie:  "picquic",
		Expires: 45 * time.Minute,
	}),
}

// Context is a custom iris.Context that will make it easy for us
// to access a client's session with a `ctx.Session()` call.
type Context struct {
	iris.Context
	session *sessions.Session
}

// Session returns the current client's session.
func (ctx *Context) Session() *sessions.Session {
	// this help us if we call `Session()` multiple times in the same handler
	if ctx.session == nil {
		// start a new session if not created before.
		ctx.session = pq.sessionsManager.Start(ctx.Context)
	}

	return ctx.session
}

// We'll use a sync.Pool to store our client Contexts
var contextPool = sync.Pool{New: func() interface{} {
	return &Context{}
}}

// Fetch a Context from our pool but replace it's Context with the original
// one that we were passed.  Clear out the session, too.
func acquire(original iris.Context) *Context {
	ctx := contextPool.Get().(*Context)
	ctx.Context = original // set the context to the original one in order to have access to iris's implementation.
	ctx.session = nil      // reset the session
	return ctx
}

// Release our Context, putting it back in the pool.
func release(ctx *Context) {
	contextPool.Put(ctx)
}

// Handler will convert our handler of func(*Context) to an iris Handler,
// in order to be compatible with the HTTP API.
func Handler(h func(*Context)) iris.Handler {
	return func(original iris.Context) {
		ctx := acquire(original)
		h(ctx)
		release(ctx)
	}
}

func limitedUpload(ctx *Context) {

	var uploads Uploads

	gob.Register(Uploads{})

	upsEncoded := ctx.Session().Get("uploads")

	if upsEncoded != nil {
		buf := bytes.NewBufferString(upsEncoded.(string))
		dec := gob.NewDecoder(buf)
		dec.Decode(&uploads)
	}

	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		ctx.Application().Logger().Warnf("Error while uploading: %v", err.Error())
		return
	}

	defer file.Close()
	fname := info.Filename

	log.Println("fname:", fname)

	uploads.Files = append(uploads.Files, fname)
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	enc.Encode(uploads)

	ctx.Session().Set("uploads", b.String())

	log.Printf("Uploads: %+v", uploads)

	f, err := os.OpenFile("./scratch/"+fname, os.O_WRONLY|os.O_CREATE, 0666)
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

	result := fmt.Sprintf("Wrote %v bytes to %v", written, fname)

	ctx.Text(result)

}

func main() {

	db, _ := boltdb.New("./db/sessions.db", 0666, "users")

	// close and unlock the database when control+C/cmd+C pressed
	iris.RegisterOnInterrupt(func() {
		db.Close()
	})

	pq.sessionsManager.UseDatabase(db)

	app := iris.New()

	app.StaticWeb("/", "./assets")

	app.Post("/upload", iris.LimitRequestBodySize(maxUploadSize), Handler(limitedUpload))

	// Start the server at http://localhost:9000
	app.Run(iris.Addr(":9000"))
}
