package main

import (
	"github.com/gin-gonic/gin"
)

func homeHandler(c *gin.Context) {
	c.String(200, "Home")
}

func pingHandler(c *gin.Context) {
	c.String(200, "Ping")
}

// loggerMiddleware is a gin middleware; it shares gin.HandlerFunc's signature but
// is not the terminal endpoint handler.
func loggerMiddleware(c *gin.Context) {
	c.Next()
}

func setupRoutes(r *gin.Engine) {
	// Verb methods with a plain function handler.
	r.GET("/", homeHandler)
	r.POST("/submit", pingHandler)

	// Closure handler.
	r.GET("/closure", func(c *gin.Context) {
		c.String(200, "Closure")
	})

	// Any registers the same handler under all verbs.
	r.Any("/any", pingHandler)

	// Handle takes the method as the first argument.
	r.Handle("PUT", "/put", pingHandler)

	// Middleware precedes the endpoint handler; only the terminal handler is the
	// endpoint.
	r.GET("/mw", loggerMiddleware, homeHandler)

	// Sub-group: routes are detected, though the group prefix ("/v1") is a known
	// limitation and is not prepended to the leaf path.
	v1 := r.Group("/v1")
	v1.GET("/users", pingHandler)
}

func main() {
	r := gin.New()
	setupRoutes(r)
	r.Run(":8080")
}
