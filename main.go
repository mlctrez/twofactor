package main

import (
	"context"
	"os"
	"time"

	"github.com/kardianos/service"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
	"github.com/mlctrez/servicego"
	"github.com/mlctrez/twofactor/server"
	"github.com/mlctrez/twofactor/ui"
)

type twoFactor struct {
	servicego.Defaults
	serverShutdown func(ctx context.Context) error
}

func main() {
	defer func() {
		app.Log("main exiting")
	}()
	app.Route("/", &ui.Body{})
	if app.IsClient {
		app.Log("run when on browser")
		app.RunWhenOnBrowser()
		return
	} else {
		servicego.Run(&twoFactor{})
	}

}

func (t *twoFactor) Start(s service.Service) (err error) {
	t.serverShutdown, err = server.Run()
	return nil
}

func (t *twoFactor) Stop(s service.Service) (err error) {
	if t.serverShutdown != nil {

		stopContext, cancel := context.WithTimeout(context.Background(), time.Second*1)
		defer cancel()

		err = t.serverShutdown(stopContext)
		if err != nil {
			_ = t.Log().Error(err)
			os.Exit(-1)
		}
	}
	return err
}
