package main

import (
	"context"
	"os"
	"time"

	"github.com/kardianos/service"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
	"github.com/mlctrez/servicego"
	tf "github.com/mlctrez/twofactor"
	"github.com/mlctrez/twofactor/server"
	"github.com/mlctrez/twofactor/ui"
)

type twoFactor struct {
	servicego.Defaults
	serverShutdown func(ctx context.Context) error
}

func main() {

	app.Route("/", &ui.Body{})

	if app.IsClient {
		app.Log("version", tf.Version, "commit", tf.Commit)
		app.RunWhenOnBrowser()
	} else {
		servicego.Run(&twoFactor{})
	}

}

func (t *twoFactor) Start(_ service.Service) (err error) {
	_ = t.Log().Infof("version %s commit %s", tf.Version, tf.Commit)
	t.serverShutdown, err = server.Run()
	return nil
}

func (t *twoFactor) Stop(_ service.Service) (err error) {
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
