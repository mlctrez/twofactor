package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dim13/otpauth/migration"
	fetch "github.com/mlctrez/wasm-fetch"
	"strconv"
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
	parameters   []*migration.Payload_OtpParameters
	progress     *progress.Circular
	updater      *demo.AppUpdateBanner
	errorMessage string
	done         chan bool
	start        int64
	end          int64
}

func (b *Body) OnAppUpdate(context app.Context) {
	if app.Getenv("DEV") != "" {
		if context.AppUpdateAvailable() {
			context.Reload()
		}
	}
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
	fmt.Println("Body Render")
	var body []app.UI
	body = append(body, b.updater, b.clipboard, b.progress)
	body = append(body, app.Span().Class("error").Text(b.errorMessage))
	body = append(body, app.Div().Class("container").Body(app.Range(b.parameters).Slice(b.renderParameterN)))

	if app.IsClient && app.Window().URL().Path == "/edit" {
		body = append(body, app.Hr(), b.edit())
	}
	body = append(body, app.Div().Class("version").Text("Version: "+twofactor.Version))

	return app.Div().Body(body...)
}

func (b *Body) edit() app.UI {
	return app.Div().Body(
		app.Label().For("startAt").Text("start"),
		app.Input().Size(5).ID("startAt").Type("text").OnInput(func(ctx app.Context, e app.Event) {
			var err error
			b.start, err = strconv.ParseInt(ctx.JSSrc().Get("value").String(), 10, 16)
			if err != nil {
				b.errorMessage = err.Error()
				return
			}
		}),
		app.Label().For("endAt").Text("end"),
		app.Input().Size(5).ID("endAt").Type("text").OnInput(func(ctx app.Context, e app.Event) {
			var err error
			b.end, err = strconv.ParseInt(ctx.JSSrc().Get("value").String(), 10, 16)
			if err != nil {
				b.errorMessage = err.Error()
				return
			}
		}),
		app.Button().Text("switch").OnClick(func(ctx app.Context, e app.Event) {
			b.storage.Switch(ctx, int(b.start), int(b.end))
			b.Update()
		}),
		app.Button().Text("delete").OnClick(func(ctx app.Context, e app.Event) {
			b.storage.Delete(ctx, int(b.start), int(b.end))
			b.parameters = b.storage.OtpParams
			b.Update()
		}),
		app.Button().Text("upload").OnClick(func(ctx app.Context, e app.Event) {
			body := &bytes.Buffer{}
			err := json.NewEncoder(body).Encode(b.storage)
			if err != nil {
				b.errorMessage = err.Error()
				return
			}
			resp, err := fetch.Fetch("/api/storage", &fetch.Opts{Method: "POST", Body: body})
			if err != nil {
				b.errorMessage = err.Error()
				return
			}
			if resp.Status != 200 {
				b.errorMessage = resp.StatusText
				return
			}
			b.errorMessage = "uploaded"
			ctx.After(2*time.Second, func(context app.Context) { b.errorMessage = "" })
		}),
		app.Button().Text("download").OnClick(func(ctx app.Context, e app.Event) {
			resp, err := fetch.Fetch("/api/storage", &fetch.Opts{})
			if err != nil {
				b.errorMessage = err.Error()
				return
			}
			if resp.Status != 200 {
				b.errorMessage = resp.StatusText
				return
			}
			downloadStorage := &store.Storage{}
			err = json.Unmarshal(resp.Body, downloadStorage)
			if err != nil {
				b.errorMessage = err.Error()
				return
			}
			store.Write(ctx, downloadStorage)
			store.Read(ctx, b.storage)
			b.parameters = b.storage.OtpParams
			b.Update()
		}),
	)
}

func (b *Body) renderParameterN(i int) app.UI {
	param := b.parameters[i]
	return app.Div().Class("row").Body(
		app.Span().Class("totp").Text(param.EvaluateString()).OnClick(func(ctx app.Context, e app.Event) {
			b.clipboard.WriteText(param.EvaluateString())
		}),
		b.renderParameterName(i),
		//app.Span().Class("name").Text(base32.StdEncoding.EncodeToString(param.Secret)),
	)
}

func (b *Body) renderParameterName(i int) app.UI {
	param := b.parameters[i]

	var name = param.Name
	if param.Issuer != "" && !strings.HasPrefix(param.Name, param.Issuer) {
		name = fmt.Sprintf("%s %s", param.Issuer, param.Name)
	}
	name = fmt.Sprintf("%02d %s", i, name)
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
	b.Update()
}
