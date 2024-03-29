//go:build !wasm

package server

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

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
	engine.RedirectTrailingSlash = false

	engine.Use(gin.Logger(), gin.Recovery())

	staticHandler := http.FileServer(http.FS(webDirectory))
	engine.GET("/web/:path", gin.WrapH(staticHandler))
	engine.GET("/api/storage", func(c *gin.Context) {
		store, se := os.ReadFile("storage.json")
		if se != nil {
			c.Status(500)
			return
		}
		c.Header("Content-Type", "application/json")
		_, we := c.Writer.Write(store)
		if we != nil {
			log.Println("error writing storage.json")
		}
	})
	engine.POST("/api/storage", func(c *gin.Context) {
		store, je := io.ReadAll(c.Request.Body)
		if je != nil {
			c.Status(500)
			return
		}
		se := os.WriteFile("storage.json", store, 0644)
		if se != nil {
			c.Status(500)
			return
		}
	})

	goAppHandler := gin.WrapH(BuildHandler())
	engine.NoRoute(func(c *gin.Context) {
		c.Writer.WriteHeader(200)
		goAppHandler(c)
	})

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
	fmt.Printf("version = %q\n", version)
	return &app.Handler{
		Author:          "mlctrez",
		Description:     "Two Factor PWA similar to google authenticator",
		Name:            "Two Factor",
		BackgroundColor: "#111",
		Icon: app.Icon{
			AppleTouch: "https://raw.githubusercontent.com/mlctrez/twofactor/master/server/web/logo-192.png",
			Default:    "https://raw.githubusercontent.com/mlctrez/twofactor/master/server/web/logo-192.png",
			Large:      "https://raw.githubusercontent.com/mlctrez/twofactor/master/server/web/logo-512.png",
			SVG:        "https://raw.githubusercontent.com/mlctrez/twofactor/master/server/web/logo.svg",
		},
		AutoUpdateInterval: updateInterval,
		ShortName:          "two factor",
		Version:            version,
		LoadingLabel:       " ",
		Styles: []string{
			"/web/app.css",
		},
		RawHeaders: []string{"<meta content=\"light dark\" name=\"color-scheme\"/>\n"},
		Title:      "Two Factor",
		Env:        map[string]string{"DEV": os.Getenv("DEV")},
	}
}
