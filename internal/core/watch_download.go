package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sources describe naming conventions, not proof that a download completed.
// Keep the whole list here so registration and final tree checks agree.
var watchDownloadSuffixes = []string{
	".crdownload",             // Chromium chrome/browser/download/download_target_determiner.cc
	".crswap",                 // Chromium content/browser/file_system_access/file_system_access_file_handle_impl.cc
	".part",                   // Firefox uriloader/exthandler/nsExternalHelperAppService.cpp
	".partial",                // Microsoft Edge/Internet Explorer partial downloads
	".download",               // Safari download bundles
	".filepart",               // WinSCP source/core/CopyParam.cpp (resumable transfers)
	".tmp",                    // Generic temporary download/staging files
	".td", ".td.cfg", ".xltd", // Xunlei/Thunder incomplete data and resume configuration
	".!qb",         // qBittorrent src/base/bittorrent/sessionimpl.cpp (incomplete extension)
	".!ut", ".!bt", // uTorrent/BitTorrent: Append .!ut/.!bt to incomplete files
	".bc!",        // BitComet: Add .bc! extension to unfinished files
	".opdownload", // Opera incomplete downloads
	".aria2",      // aria2 src/DefaultBtProgressInfoFile.h getSuffix (X.aria2 resumes X)
}

func watchDownload(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	for _, suffix := range watchDownloadSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	// IDM's temporary directory is DwnlData/<user>/<download parts> (IDM
	// Options > Save to > Temporary directory). Protect it and its descendants.
	for dir := path; ; dir = filepath.Dir(dir) {
		if strings.EqualFold(filepath.Base(dir), "DwnlData") {
			return true
		}
		if filepath.Dir(dir) == dir {
			return false
		}
	}
}

func checkWatchDownload(path string) error {
	if watchDownload(path) {
		return fmt.Errorf("download temporary name is still present: %s", path)
	}
	// Enumerate to cover case variants even in case-sensitive directories.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		return err
	}
	return watchSidecar(path, entries)
}

func watchSidecar(path string, entries []os.DirEntry) error {
	want := filepath.Base(path) + ".aria2"
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), want) {
			return fmt.Errorf("download resume sidecar is still present: %s", filepath.Join(filepath.Dir(path), entry.Name()))
		}
	}
	return nil
}

type WatchSkip struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	Manage string `json:"manage"`
}

// WatchDownloadSkips is an on-demand diagnostic, not persisted scan state.
// Callers verify the policy directory before using its current entries.
func WatchDownloadSkips(path string) ([]WatchSkip, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var skipped []WatchSkip
	for _, e := range entries {
		child := filepath.Join(path, e.Name())
		var why error
		if watchDownload(child) {
			why = fmt.Errorf("download temporary name is still present: %s", child)
		} else {
			why = watchSidecar(child, entries)
		}
		if why != nil {
			skipped = append(skipped, WatchSkip{Path: child, Reason: "因为像下载临时文件而跳过：" + why.Error(), Manage: fmt.Sprintf("obit add %q <lifespan>", child)})
		}
	}
	return skipped, nil
}
