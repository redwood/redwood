//go:build webview
package main

import (
	"sync"

    "github.com/webview/webview"
)

type GUI struct {
	closeOnce sync.Once
	chDone    chan struct{}
	api *API
	webview webview.WebView
}

func newGUI(api *API) *GUI {
	return &GUI{
		api:    api,
		chDone: make(chan struct{}),
	}
}

func (gui *GUI) Start() error {
	defer close(gui.chDone)
	debug := true
	gui.webview = webview.New(debug)
	gui.webview.SetTitle("Minimal webview example")
	gui.webview.SetSize(800, 600, webview.HintNone)
	gui.webview.Navigate(fmt.Sprintf("http://localhost:%v/index.html", gui.api.port))
	gui.webview.Run()
	return nil


func (gui *GUI) Close() (err error) {
	gui.closeOnce.Do(func() {
	 gui.webview.Destroy()
	 gui.webview.Dispatch(func() {
	     gui.webview.Destroy()
	     gui.webview.Terminate()
	 })
	})
	return nil
}
