// Package sessions contains middleware for easy session management in Martini.
//
//  package main
//
//  import (
//    "github.com/go-martini/martini"
//    "github.com/martini-contrib/sessions"
//  )
//
//  func main() {
// 	  m := martini.Classic()
//
// 	  store := sessions.NewCookieStore([]byte("secret123"))
// 	  m.Use(sessions.Sessions("my_session", store))
//
// 	  m.Get("/", func(session sessions.Session) string {
// 		  session.Set("hello", "world")
// 	  })
//  }
package sessions

import (
	"log"
	"net/http"

	"github.com/go-martini/martini"
	"github.com/gorilla/context"
	"github.com/gorilla/sessions"
)

const (
	errorFormat = "[sessions] ERROR! %s\n"
)

// Store is an interface for custom session stores.
type Store interface {
	sessions.Store
}

// Options stores configuration for a session or session store.
//
// Fields are a subset of http.Cookie fields.
type Options struct {
	Path   string
	Domain string
	// MaxAge=0 means no 'Max-Age' attribute specified.
	// MaxAge<0 means delete cookie now, equivalently 'Max-Age: 0'.
	// MaxAge>0 means Max-Age attribute present and given in seconds.
	MaxAge   int
	Secure   bool
	HttpOnly bool
}

var _ Session = (*session)(nil)

// Session stores the values and optional configuration for a session.
type Session interface {
	// Get returns the session value associated to the given key.
	Get(name string, key interface{}) interface{}
	// Set sets the session value associated to the given key.
	Set(name string, key interface{}, val interface{})
	// Delete removes the session value associated to the given key.
	Delete(name string, key interface{})
	// Clear deletes all values in the session.
	Clear(name string)
	// AddFlash adds a flash message to the session.
	// A single variadic argument is accepted, and it is optional: it defines the flash key.
	// If not defined "_flash" is used by default.
	AddFlash(name string, value interface{}, vars ...string)
	// Flashes returns a slice of flash messages from the session.
	// A single variadic argument is accepted, and it is optional: it defines the flash key.
	// If not defined "_flash" is used by default.
	Flashes(name string, vars ...string) []interface{}
	// Options sets confuguration for a session.
	Options(name string, opts Options)
}

// Sessions is a Middleware that maps a session.Session service into the Martini handler chain.
// Sessions can use a number of storage solutions with the given store.
func Sessions(store Store) martini.Handler {
	return func(res http.ResponseWriter, r *http.Request, c martini.Context, l *log.Logger) {
		// Map to the Session interface
		s := &session{
			ss:      make(map[string]*sessions.Session),
			written: make(map[string]bool),
			request: r,
			store:   store,
			logger:  l,
		}
		c.MapTo(s, (*Session)(nil))

		// Use before hook to save out the session
		rw := res.(martini.ResponseWriter)
		rw.Before(func(martini.ResponseWriter) {
			for n := range s.ss {
				if s.Written(n) {
					check(s.Session(n).Save(r, res), l)
				}
			}
		})

		// clear the context, we don't need to use
		// gorilla context and we don't want memory leaks
		defer context.Clear(r)

		c.Next()
	}
}

type session struct {
	ss      map[string]*sessions.Session
	written map[string]bool
	request *http.Request
	logger  *log.Logger
	store   Store
}

func (s *session) Get(name string, key interface{}) interface{} {
	return s.Session(name).Values[key]
}

func (s *session) Set(name string, key interface{}, val interface{}) {
	s.Session(name).Values[key] = val
	s.written[name] = true
}

func (s *session) Delete(name string, key interface{}) {
	delete(s.Session(name).Values, key)
	s.written[name] = true
}

func (s *session) Clear(name string) {
	for key := range s.Session(name).Values {
		s.Delete(name, key)
	}
}

func (s *session) AddFlash(name string, value interface{}, vars ...string) {
	s.Session(name).AddFlash(value, vars...)
	s.written[name] = true
}

func (s *session) Flashes(name string, vars ...string) []interface{} {
	s.written[name] = true
	return s.Session(name).Flashes(vars...)
}

func (s *session) Options(name string, options Options) {
	s.Session(name).Options = &sessions.Options{
		Path:     options.Path,
		Domain:   options.Domain,
		MaxAge:   options.MaxAge,
		Secure:   options.Secure,
		HttpOnly: options.HttpOnly,
	}
}

func (s *session) Session(name string) *sessions.Session {
	if s.ss[name] == nil {
		var err error
		s.ss[name], err = s.store.Get(s.request, name)
		check(err, s.logger)
	}

	return s.ss[name]
}

func (s *session) Written(name string) bool {
	return s.written[name]
}

func check(err error, l *log.Logger) {
	if err != nil {
		l.Printf(errorFormat, err)
	}
}
