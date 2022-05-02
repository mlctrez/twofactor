package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/dim13/otpauth/migration"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
	"github.com/mlctrez/goapp-mdc/pkg/progress"
	"github.com/mlctrez/imgtofactbp/components/clipboard"
	"github.com/mlctrez/imgtofactbp/conversions"
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
	errorMessage string
	done         chan bool
}

func (b *Body) OnInit() {
	app.Log("OnInit")
	b.clipboard = &clipboard.Clipboard{ID: "clipboard"}
	b.progress = progress.NewCircular("progress", 64)
	b.storage = &store.Storage{}
	b.done = make(chan bool, 2)
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
				thirties := now.Second() % 30
				val := 1 - (float64(thirties) / 30)
				b.progress.SetProgress(val)
				if thirties == 0 {
					app.Log("update")
					b.Update()
				}
			})
		}
	}
}

func (b *Body) OnMount(ctx app.Context) {
	app.Log("OnMount")
	ctx.Handle("Clipboard:paste", b.imagePaste)
	b.parameters = store.Read(ctx, b.storage)
	b.progress.Open()
	ctx.Async(func() { b.progressLoop(ctx) })
}

func (b *Body) OnDismount() {
	b.done <- true
	close(b.done)
}

func (b *Body) Render() app.UI {
	app.Log("Render")
	return app.Div().Body(
		b.clipboard,
		b.progress,
		app.Div().Class("container").Body(app.Range(b.parameters).Slice(b.renderParameterN)),
	)
}

func (b *Body) renderParameterN(i int) app.UI {
	param := b.parameters[i]
	return app.Div().Class("row").Body(
		app.Span().Class("totp").Text(param.EvaluateString()),
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
	if err != nil {
		return
	}
	ctx.Dispatch(func(context app.Context) {
		b.errorMessage = err.Error()
	})
}

func (b *Body) imagePaste(ctx app.Context, action app.Action) {
	data, ok := action.Value.(*clipboard.PasteData)
	if !ok {
		return
	}
	img, _, err := conversions.Base64ToImage(data.Data)
	if err != nil {
		b.setError(ctx, err)
		return
	}
	bmp, _ := gozxing.NewBinaryBitmapFromImage(img)
	qrReader := qrcode.NewQRCodeReader()
	result, err := qrReader.Decode(bmp, nil)
	if err != nil {
		b.setError(ctx, err)
		return
	}
	payload, err := migration.UnmarshalURL(result.GetText())
	if err != nil {
		b.setError(ctx, err)
		return
	}
	err = b.storage.Add(payload)
	if err != nil {
		b.setError(ctx, err)
		return
	}
	b.parameters = store.Write(ctx, b.storage)
}
