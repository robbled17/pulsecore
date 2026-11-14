//go:build !windows

package main

func (a *App) CleanupOldBinary() {}

func (a *App) CheckForUpdates() (UpdateInfo, error) {
	return UpdateInfo{Available: false}, nil
}

func (a *App) DownloadAndInstallUpdate(downloadURL string, expectedDigest string) error {
	return nil
}
