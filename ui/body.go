package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dim13/otpauth/migration"
	fetch "github.com/mlctrez/wasm-fetch"

	"github.com/maxence-charriere/go-app/v9/pkg/app"
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
	errorMessage string
	done         chan bool
	manualName   string
	manualSecret string
	start        int64
	end          int64
	timer        string
	editIndex    int
	editIssuer   string
	editName     string
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
	b.storage = &store.Storage{}
	b.done = make(chan bool, 2)
	b.editIndex = -1
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
				b.updateProgress(now)
			})
		}
	}
}

func (b *Body) updateProgress(now time.Time) {
	thirties := now.Add(5*time.Second).Second() % 30
	b.timer = fmt.Sprintf("%d", 30-thirties)
	b.Update()
}

func (b *Body) OnMount(ctx app.Context) {
	ctx.Handle("Clipboard:paste", b.clipboardPaste)
	b.parameters = store.Read(ctx, b.storage)
	b.updateProgress(time.Now())
	ctx.Async(func() { b.progressLoop(ctx) })
}

func (b *Body) OnDismount() {
	b.done <- true
	close(b.done)
}

func (b *Body) Render() app.UI {
	var body []app.UI

	body = append(body, app.Span().Class("timer").Text(b.timer), app.Br(), b.clipboard)
	body = append(body, app.Span().Class("error").Text(b.errorMessage))
	body = append(body, app.Div().Class("container").Body(app.Range(b.parameters).Slice(b.renderParameterN)))

	isEdit := app.IsClient && app.Window().URL().Path == "/edit"
	if isEdit {
		body = append(body, app.Hr(), b.edit())
	}
	body = append(body, app.Div().OnClick(func(ctx app.Context, e app.Event) {
		if isEdit {
			app.Window().Get("location").Set("href", "/")
		} else {
			app.Window().Get("location").Set("href", "/edit")
		}
		b.Update()
	}).Class("version").Text("Version: "+twofactor.Version))

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
			b.cancelRename()
			b.storage.Switch(ctx, int(b.start), int(b.end))
			b.Update()
		}),
		app.Button().Text("delete").OnClick(func(ctx app.Context, e app.Event) {
			b.cancelRename()
			b.storage.Delete(ctx, int(b.start), int(b.end))
			b.parameters = b.storage.OtpParams
			b.Update()
		}),
		app.Hr(),
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
			b.cancelRename()
			store.Write(ctx, downloadStorage)
			store.Read(ctx, b.storage)
			b.parameters = b.storage.OtpParams
			b.Update()
		}),
		app.Hr(),
		app.Label().For("manualName").Text("name"),
		app.Input().ID("manualName").Type("text").Value(b.manualName).OnInput(func(ctx app.Context, e app.Event) {
			b.manualName = ctx.JSSrc().Get("value").String()
		}),
		app.Label().For("manualSecret").Text("secret"),
		app.Input().ID("manualSecret").Type("text").Value(b.manualSecret).OnInput(func(ctx app.Context, e app.Event) {
			b.manualSecret = ctx.JSSrc().Get("value").String()
		}),
		app.Button().Text("add").OnClick(func(ctx app.Context, e app.Event) {
			err := b.storage.AddSecret(b.manualName, b.manualSecret)
			if err != nil {
				b.errorMessage = err.Error()
			} else {
				b.manualName = ""
				b.manualSecret = ""
				b.parameters = b.storage.OtpParams
				store.Write(ctx, b.storage)
			}
			b.Update()
		}),
	)
}

func (b *Body) renderParameterN(i int) app.UI {
	param := b.parameters[i]
	isEdit := app.IsClient && app.Window().URL().Path == "/edit"
	row := []app.UI{
		app.Span().Class("totp").Text(param.EvaluateString()).OnClick(func(ctx app.Context, e app.Event) {
			b.clipboard.WriteText(param.EvaluateString())
		}),
	}
	switch {
	case isEdit && b.editIndex == i:
		row = append(row, b.renderRename(i))
	case isEdit:
		row = append(row, b.renderParameterName(i),
			app.Button().Text("rename").OnClick(func(ctx app.Context, e app.Event) {
				b.startRename(i)
			}))
	default:
		row = append(row, b.renderParameterName(i))
	}
	//app.Span().Class("name").Text(base32.StdEncoding.EncodeToString(param.Secret)),
	return app.Div().Class("row").Body(row...)
}

func (b *Body) startRename(i int) {
	if i < 0 || i >= len(b.parameters) {
		return
	}
	b.editIndex = i
	b.editIssuer = b.parameters[i].Issuer
	b.editName = b.parameters[i].Name
	b.errorMessage = ""
	b.Update()
}

func (b *Body) cancelRename() {
	b.editIndex = -1
	b.editIssuer = ""
	b.editName = ""
}

func (b *Body) renderRename(i int) app.UI {
	issuerID := fmt.Sprintf("renameIssuer%d", i)
	nameID := fmt.Sprintf("renameName%d", i)
	return app.Span().Class("name").Body(
		app.Text(fmt.Sprintf("%02d ", i)),
		app.Label().For(issuerID).Text("issuer"),
		app.Input().ID(issuerID).Type("text").Value(b.editIssuer).OnInput(func(ctx app.Context, e app.Event) {
			b.editIssuer = ctx.JSSrc().Get("value").String()
		}),
		app.Label().For(nameID).Text("name"),
		app.Input().ID(nameID).Type("text").Value(b.editName).OnInput(func(ctx app.Context, e app.Event) {
			b.editName = ctx.JSSrc().Get("value").String()
		}),
		app.Button().Text("save").OnClick(func(ctx app.Context, e app.Event) {
			if err := b.storage.Rename(i, b.editIssuer, b.editName); err != nil {
				b.errorMessage = err.Error()
				b.Update()
				return
			}
			b.parameters = store.Write(ctx, b.storage)
			b.cancelRename()
			b.errorMessage = ""
			b.Update()
		}),
		app.Button().Text("cancel").OnClick(func(ctx app.Context, e app.Event) {
			b.cancelRename()
			b.errorMessage = ""
			b.Update()
		}),
	)
}

func (b *Body) renderParameterName(i int) app.UI {
	isEdit := app.IsClient && app.Window().URL().Path == "/edit"
	param := b.parameters[i]

	name := store.Label(param)
	if isEdit {
		name = fmt.Sprintf("%02d %s", i, name)
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
	activeElement := app.Window().Get("document").Get("activeElement")
	if activeElement.Truthy() {
		tagName := strings.ToLower(activeElement.Get("tagName").String())
		if tagName == "input" || tagName == "textarea" {
			return
		}
	}

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
