//go:build !windows

package main

func invalidateMainHWNDCache() {}

// TitlebarDragBegin stub for non-windows.
func (a *App) TitlebarDragBegin() {}

// TitlebarToggleMaximise stub for non-windows.
func (a *App) TitlebarToggleMaximise() {}

// WindowMaximisedState stub for non-windows.
func (a *App) WindowMaximisedState() bool {
	return false
}
