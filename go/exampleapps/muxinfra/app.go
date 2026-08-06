package main

import (
	"net/http"
)

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Home"))
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Ping"))
}

// group holds the route-registration methods. It mirrors real frameworks (e.g.
// gin, whose verb methods live on an embedded *gin.RouterGroup) where the
// registration methods are promoted from an embedded sub-struct. A call on the
// outer router is therefore lowered by the compiler into a call on a
// field-addressed inner value.
type group struct{}

// HandleFunc registers a plain function handler.
func (g *group) HandleFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {}

// Handle takes an interface, mirroring net/http.ServeMux.Handle: passing an
// http.HandlerFunc-boxed function makes the SSA arg an *ssa.MakeInterface.
func (g *group) Handle(pattern string, h http.Handler) {}

// router embeds group by value, so router.HandleFunc(...) is promoted to
// (&router.group).HandleFunc(...). The receiver is a field-addressed *group — a
// router-typed value derived from the original router whose referrer is the
// registration call. This is the exact shape the value-following logic handles.
type router struct {
	group
}

func newRouter() *router {
	return &router{}
}

// setupRoutes registers handlers through the embedded (promoted) group methods.
func setupRoutes() *router {
	r := newRouter()

	// Promoted registration -> exercises router-value-following (the receiver is
	// a derived *group value reached via a FieldAddr on the router).
	r.HandleFunc("/promoted", homeHandler)

	// Interface-boxed handler -> exercises the ssa.MakeInterface case in the
	// handler parser (http.HandlerFunc(fn) boxed to http.Handler).
	r.Handle("/boxed", http.HandlerFunc(pingHandler))

	return r
}

func main() {
	_ = setupRoutes()
}
