package main

// AppVersion is the user-visible PulseCore release.
const AppVersion = "1.2.0"

// GetAppVersion returns the current application version string.
func (a *App) GetAppVersion() string {
	return AppVersion
}
