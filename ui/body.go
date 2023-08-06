package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/maxence-charriere/go-app/v9/pkg/app"
	"github.com/mlctrez/goapp-mdc-demo/demo"
	"github.com/mlctrez/goapp-mdc/pkg/progress"
	"github.com/mlctrez/imgtofactbp/components/clipboard"
	twofactor "github.com/mlctrez/twofactor"
	"github.com/mlctrez/twofactor/store"
)

var _ app.Mounter = (*Body)(nil)
var _ app.Dismounter = (*Body)(nil)
var _ app.Initializer = (*Body)(nil)

type Body struct {
	app.Compo
	clipboard    *clipboard.Clipboard
	storage      *store.Storage
	parameters   []*store.Parameter
	progress     *progress.Circular
	updater      *demo.AppUpdateBanner
	errorMessage string
	done         chan bool
}

func (b *Body) OnInit() {
	b.clipboard = &clipboard.Clipboard{ID: "clipboard"}
	b.progress = progress.NewCircular("progress", 64)
	b.storage = &store.Storage{}
	b.done = make(chan bool, 2)
	b.updater = &demo.AppUpdateBanner{}
}

func (b *Body) progressLoop(ctx app.Context) {
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-b.done:
			ticker.Stop()
			return
		case now := <-ticker.C:
			ctx.Defer(func(context app.Context) {
				thirties := b.updateProgress(now)
				if thirties == 0 {
					b.Update()
				}
			})
		}
	}
}

func (b *Body) updateProgress(now time.Time) int {
	thirties := now.Second() % 30
	val := 1 - (float64(thirties) / 30)
	b.progress.SetProgress(val)
	return thirties
}

func (b *Body) OnMount(ctx app.Context) {
	ctx.Handle("Clipboard:paste", b.clipboardPaste)
	b.parameters = store.Read(ctx, b.storage)
	b.updateProgress(time.Now())
	b.progress.Open()
	ctx.Async(func() { b.progressLoop(ctx) })
}

func (b *Body) OnDismount() {
	b.done <- true
	close(b.done)
}

func (b *Body) Render() app.UI {
	return app.Div().Body(
		b.updater,
		b.clipboard,
		b.progress,
		app.If(b.errorMessage != "", app.Span().Class("error").Text(b.errorMessage)),
		app.Div().Class("container").Body(app.Range(b.parameters).Slice(b.renderParameterN)),
		app.Div().Class("version").Text("Version: "+twofactor.Version),
	)
}

func (b *Body) renderParameterN(i int) app.UI {
	param := b.parameters[i]
	return app.Div().Class("row").Body(
		app.Span().Class("totp").Text(param.EvaluateString()).OnClick(func(ctx app.Context, e app.Event) {
			b.clipboard.WriteText(param.EvaluateString())
		}),
		b.renderParameterName(i),
	)
}

func (b *Body) renderParameterName(i int) app.UI {
	param := b.parameters[i]

	var name = param.Name
	if param.Issuer != "" && !strings.HasPrefix(param.Name, param.Issuer) {
		name = fmt.Sprintf("%s %s", param.Issuer, param.Name)
	}
	return app.Span().Class("name").Text(name)
}

func (b *Body) setError(ctx app.Context, err error) {
	var errorMessage string
	if err == nil {
		errorMessage = ""
	} else {
		errorMessage = err.Error()
	}
	ctx.Dispatch(func(context app.Context) {
		if errorMessage != "" {
			app.Log(errorMessage)
		}
		b.errorMessage = errorMessage
	})
	if errorMessage != "" {
		ctx.After(time.Second*20, func(context app.Context) {
			b.setError(context, nil)
		})
	}
}

func (b *Body) clipboardPaste(ctx app.Context, action app.Action) {
	data, ok := action.Value.(*clipboard.PasteData)
	if !ok {
		return
	}
	if !strings.HasPrefix(data.Data, "data:image") {
		b.setError(ctx, errors.New("pasting text not supported"))
		return
	}

	err := b.storage.Paste(data)
	if err != nil {
		b.setError(ctx, err)
		return
	}
	b.parameters = store.Write(ctx, b.storage)
}
