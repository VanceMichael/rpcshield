package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func wrapLogger(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Logging")
		h(w, r)
	}
}

func wrapLoggerTricky(a http.HandlerFunc, b http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a(w, r)
		b(w, r)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Home"))
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Ping"))
}

func addMoreRoutes(router *mux.Router) {
	router.HandleFunc("/more/ping", pingHandler)
	addSomeMoreRoutes(router)
}

func addSomeMoreRoutes(router *mux.Router) {
	router.HandleFunc("/some/more/ping", pingHandler)

	// add dummy infinite loop to check that the parser halts
	addMoreRoutes(router)
}

// CustomHandler0 is a handler to test HTTP IB endpoint extraction
type CustomHandler0 struct{}

// NewCustomHandler0 creates a CustomHandler0
func NewCustomHandler0() *CustomHandler0 {
	return &CustomHandler0{}
}

func (c *CustomHandler0) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Custom Handler 0"))
}

// CustomHandlerInterface defines the interface for custom HTTP handlers.
type CustomHandlerInterface interface {
	Create(w http.ResponseWriter, r *http.Request)
	Delete(w http.ResponseWriter, r *http.Request)
}

// CustomHandler1 implements CustomHandlerInterface for handler testing.
type CustomHandler1 struct{}

// Create handles HTTP create requests for CustomHandler1.
func (c *CustomHandler1) Create(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Create Custom Handler 1"))
}

// Delete handles HTTP delete requests for CustomHandler1.
func (c *CustomHandler1) Delete(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Delete Custom Handler 1"))
}

func (c *CustomHandler1) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ServeHTTP Custom Handler 1"))
}

// CustomHandler2 implements CustomHandlerInterface for handler testing.
type CustomHandler2 struct{}

// Create handles HTTP create requests for CustomHandler2.
func (c *CustomHandler2) Create(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Create Custom Handler 2"))
}

// Delete handles HTTP delete requests for CustomHandler2.
func (c *CustomHandler2) Delete(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Delete Custom Handler 2"))
}

func (c *CustomHandler2) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ServeHTTP Custom Handler 2"))
}

func makeHandler(c CustomHandlerInterface) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		c.Create(w, r)
	}
}

func setupRoutes(c1 CustomHandler1, ciface CustomHandlerInterface) *mux.Router {
	router := mux.NewRouter()

	/* === HandleFunc test cases === */

	// Function
	router.HandleFunc("/", homeHandler)

	// Closure
	router.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Closure"))
	})

	// Wrapped
	router.HandleFunc("/debug/wrapped", wrapLogger(homeHandler))
	router.HandleFunc("/debug/wrapped/closure", wrapLogger(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Wrapped closure"))
	}))
	router.HandleFunc("/debug/wrapped/tricky", wrapLoggerTricky(homeHandler, pingHandler))

	// Local
	clos0 := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Closure"))
	}
	clos1 := clos0
	router.HandleFunc("/local/clos0", clos0)
	router.HandleFunc("/local/clos1", clos1)

	// Interface method
	router.HandleFunc("/create_iface", ciface.Create)
	router.HandleFunc("/delete_iface", ciface.Delete)
	router.HandleFunc("/create", c1.Create)
	router.HandleFunc("/delete", c1.Delete)

	// Simple constant propagation (SSA construction does some of it for us)
	const basePath = "/api/v2"
	router.HandleFunc(basePath+"/ping", pingHandler)

	// This DOES NOT get propagated. Not handled yet.
	otherBasePath := "/api/v3"
	router.HandleFunc(otherBasePath+"/ping", pingHandler)

	// Check if we can detect routes through conditionals
	if time.Now().Hour() < 12 {
		router.HandleFunc("/mystery/handler", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Mystery"))
		})
	}

	// Dynamic routes, should currently return "unknown". Not handled yet.
	endpoints := []string{"/foo", "/bar", "/baz"}
	for _, endpoint := range endpoints {
		router.HandleFunc(endpoint, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(endpoint))
		})
	}

	// Wrapped builder
	router.HandleFunc("/builder/create", makeHandler(ciface))

	/* === Handler struct test cases. Not handled right now === */

	// Handler struct. Not handled right now.
	router.Handle("/custom", &CustomHandler0{})

	c0 := NewCustomHandler0()
	router.Handle("/custom/alias", c0)

	router.Handle("/custom/iface", &c1)

	/* === Other patterns === */

	// Delegation. Not handled yet
	addMoreRoutes(router)

	return router
}

func main() {
	c1 := CustomHandler1{}
	ciface := &CustomHandler2{}
	router := setupRoutes(c1, ciface)
	http.ListenAndServe(":8080", router)
}
