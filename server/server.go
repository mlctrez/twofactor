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
	"time"

	brotli "github.com/anargu/gin-brotli"
	"github.com/gin-gonic/gin"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
	twofactor "github.com/mlctrez/twofactor"
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

	engine.Use(gin.Logger(), gin.Recovery(), brotli.Brotli(brotli.DefaultCompression))

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
	updateInterval := time.Hour * 24
	if os.Getenv("DEV") != "" {
		updateInterval = time.Second * 3
		twofactor.Version = ""
	}
	version := twofactor.Version
	return &app.Handler{
		Author:          "mlctrez",
		Description:     "Two Factor PWA similar to google authenticator",
		Name:            "Two Factor",
		BackgroundColor: "#111",
		Scripts: []string{
			"https://cdnjs.cloudflare.com/ajax/libs/material-components-web/13.0.0/material-components-web.js",
		},
		Icon: app.Icon{
			AppleTouch: "/web/logo-192.png",
			Default:    "/web/logo-192.png",
			Large:      "/web/logo-512.png",
		},
		AutoUpdateInterval: updateInterval,
		ShortName:          "two factor",
		Version:            version,
		Styles: []string{
			"https://fonts.googleapis.com/icon?family=Material+Icons",
			"https://fonts.googleapis.com/css2?family=Roboto&display=swap",
			"https://cdnjs.cloudflare.com/ajax/libs/material-components-web/13.0.0/material-components-web.css",
			"/web/app.css",
		},
		Title: "Two Factor",
	}
}
