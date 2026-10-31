package main

import (
	"context"
	"os/exec"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/yusefmosiah/go-choir/internal/tapefollow"
)

// installSyncTray puts the sync status in the menu bar ("Synced to event N,
// <time>", or the error) with "Sync now" and "Show local copy in Finder", and
// starts the syncer once the app is running. The returned func stops it.
func installSyncTray(app *application.App, ds *desktopSync) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	tray := app.SystemTray.New()
	tray.SetLabel("Choir")
	menu := application.NewMenu()
	status := menu.Add("Not synced yet")
	status.SetEnabled(false)
	menu.AddSeparator()
	menu.Add("Sync now").OnClick(func(*application.Context) { ds.SyncNow() })
	menu.Add("Show local copy in Finder").OnClick(func(*application.Context) {
		if dir := ds.StoreDir(); dir != "" {
			_ = exec.Command("/usr/bin/open", dir).Start()
		}
	})
	tray.SetMenu(menu)
	ds.onStatus = func(tapefollow.Status) {
		line := ds.StatusLine()
		application.InvokeAsync(func() { status.SetLabel(line) })
		tray.SetTooltip(line)
	}
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go ds.Run(ctx)
	})
	return cancel
}
