package main

// UpdateInfo holds details about an available PulseCore application update.
type UpdateInfo struct {
	Available    bool    `json:"available"`
	DownloadURL  string  `json:"downloadUrl"`
	SizeMB       float64 `json:"sizeMb"`
	PublishedAt  string  `json:"publishedAt"`
	ReleaseNotes string  `json:"releaseNotes"`
	Digest       string  `json:"digest"`
}

type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt string        `json:"published_at"`
	Assets      []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	UpdatedAt          string `json:"updated_at"`
	Digest             string `json:"digest,omitempty"`
}
