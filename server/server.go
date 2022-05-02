//go:build !wasm

package server

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

//go:embed web/*
var webDirectory embed.FS

func Run() (shutdownFunc func(ctx context.Context) error, err error) {

	address := os.Getenv("ADDRESS")
	if address == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "9000"
		}
		address = ":" + port
	}

	var listener net.Listener
	if listener, err = net.Listen("tcp", address); err != nil {
		return nil, err
	}
	fmt.Printf("starting server http://localhost%s\n\n", address)

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	//engine.Use(gin.Logger(), gin.Recovery(), brotli.Brotli(brotli.DefaultCompression))
	engine.Use(gin.Logger(), func(c *gin.Context) {
		c.Next()
		log.Println("completed", c.Request.RequestURI)
	})

	staticHandler := http.FileServer(http.FS(webDirectory))
	engine.GET("/web/:path", gin.WrapH(staticHandler))

	engine.NoRoute(gin.WrapH(BuildHandler()))
	//engine.RedirectTrailingSlash = false

	server := &http.Server{Handler: engine}

	go func() {
		serveErr := server.Serve(listener)
		if serveErr != nil && serveErr != http.ErrServerClosed {
			log.Println(err)
		}
	}()

	return server.Shutdown, nil
}

func BuildHandler() *app.Handler {
	return &app.Handler{
		Author:          "mlctrez",
		Description:     "Two Factor PWA",
		Name:            "Two Factor",
		BackgroundColor: "#111",
		Scripts: []string{
			"https://cdnjs.cloudflare.com/ajax/libs/material-components-web/13.0.0/material-components-web.js",
		},
		ShortName: "goapp-mdc",
		Styles: []string{
			"https://fonts.googleapis.com/icon?family=Material+Icons",
			"https://fonts.googleapis.com/css2?family=Roboto&display=swap",
			"https://cdnjs.cloudflare.com/ajax/libs/material-components-web/13.0.0/material-components-web.css",
			"/web/app.css",
		},
		Title: "Two Factor",
	}
}
