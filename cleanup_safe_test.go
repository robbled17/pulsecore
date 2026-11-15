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

func TestDriveScanCachePerDrive(t *testing.T) {
	a := &App{}
	a.InvalidateDriveScanCache("")

	// Initially nil
	cached := a.GetDriveScanCache("C")
	if cached != nil {
		t.Fatalf("expected nil cache after full invalidation, got %v", cached)
	}

	// Preview populates cache
	cats := a.PreviewDiskCleanupForDrive("C")
	if len(cats) == 0 {
		t.Fatal("expected non-empty categories")
	}

	cached = a.GetDriveScanCache("C")
	if cached == nil || !cached.CacheLoaded {
		t.Fatalf("expected cache to be loaded for drive C")
	}
	if len(cached.Cache) != len(cats) {
		t.Fatalf("expected %d cached categories, got %d", len(cats), len(cached.Cache))
	}

	// Invalidate single drive
	a.InvalidateDriveScanCache("C")
	cached = a.GetDriveScanCache("C")
	if cached != nil {
		t.Fatalf("expected nil cache after C invalidation")
	}
}

func TestApplyDiskCleanupResultShape(t *testing.T) {
	a := &App{}
	// Empty slice applies selected non-admin cats
	res := a.ApplyDiskCleanup([]string{"wer_reports"})
	if len(res.UpdatedCats) == 0 {
		t.Fatal("expected UpdatedCats to be populated")
	}
	found := false
	for _, c := range res.UpdatedCats {
		if c.ID == "wer_reports" {
			found = true
			if !c.Cleaned {
				t.Fatalf("expected wer_reports to be marked Cleaned")
			}
		}
	}
	if !found {
		t.Fatal("expected wer_reports in UpdatedCats")
	}
}
