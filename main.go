package main

import (
	"sync"
	"time"

	"github.com/kataras/iris"
	"github.com/kataras/iris/sessions"
	"github.com/kataras/iris/sessions/sessiondb/boltdb"
)

const maxUploadSize int64 = 20 * 1024 * 1024

// Uploads holds a slice of Files uploaded during this session
type Uploads struct {
	Files []File
}

// File refers to one uploaded file
type File struct {
	Name         string
	OriginalName string
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

func main() {

	db, _ := boltdb.New("./db/sessions.db", 0666, "users")

	// close and unlock the database when control+C/cmd+C pressed
	iris.RegisterOnInterrupt(func() {
		db.Close()
	})

	pq.sessionsManager.UseDatabase(db)

	app := iris.New()

	app.StaticWeb("/", "./assets")

	app.Post("/upload", iris.LimitRequestBodySize(maxUploadSize), Handler(uploadImage))

	app.Post("/delete", Handler(deleteImage))

	// Start the server at http://localhost:9000
	app.Run(iris.Addr(":9000"))
}
