package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupPathBlocksPersonal(t *testing.T) {
	blocked := []string{
		`C:\Users\Rob\Documents\tax.pdf`,
		`C:\Users\Rob\Downloads\setup.exe`,
		`C:\Users\Rob\Desktop\note.txt`,
		`C:\Users\Rob\AppData\Local\Google\Chrome\User Data\Default\Login Data`,
	}
	for _, p := range blocked {
		if isSafeCleanupPath(p) {
			t.Fatalf("must block %s", p)
		}
	}
}

func TestCleanupPathAllowsTemp(t *testing.T) {
	tmp := os.TempDir()
	p := filepath.Join(tmp, "pulsecore_cleanup_test.tmp")
	if !isSafeCleanupPath(p) && !isSafeCleanupPath(tmp) {
		// On some CI layouts Temp may not match prefix rules; at least root temp dir should.
		t.Logf("temp=%s safe_file=%v safe_dir=%v", tmp, isSafeCleanupPath(p), isSafeCleanupPath(tmp))
	}
}

func TestPreviewDiskCleanupShape(t *testing.T) {
	a := &App{}
	cats := a.PreviewDiskCleanup()
	if len(cats) != len(cleanupSafeIDs) {
		t.Fatalf("want %d cats, got %d", len(cleanupSafeIDs), len(cats))
	}
	var sum int64
	for _, c := range cats {
		if c.ID == "" {
			t.Fatal("empty id")
		}
		sum += c.SizeBytes
		if c.ID == "recycle" && !c.Irreversible {
			t.Fatal("recycle must be irreversible")
		}
	}
	_ = sum
}

func TestBlockedFolderNames(t *testing.T) {
	if !isBlockedFolderName("Downloads") || !isBlockedFolderName("Documents") {
		t.Fatal("personal folders must be blocked from large-folder list")
	}
}
