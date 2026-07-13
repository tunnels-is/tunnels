package client

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tunnels-is/tunnels/types"
	"github.com/tunnels-is/tunnels/version"
)

const (
	archive           = "update.archive"
	nextVersionSuffix = ".next"
	prevVersionSuffix = ".prev"
	repo              = "tunnels"
	owner             = "tunnels-is"

	// maxUpdateArchiveSize caps the release download and the extracted binary
	// so a compromised/misbehaving release endpoint cannot fill the disk.
	maxUpdateArchiveSize = 512 * 1024 * 1024
	// maxUpdateMetaSize caps the release-metadata and checksum responses.
	maxUpdateMetaSize = 10 * 1024 * 1024
)

// updateMetaClient fetches release metadata / checksums; updateFetchClient
// downloads the archive itself (generous timeout for slow links). The default
// http.Client has no timeout at all — a stalled connection would hang the
// updater forever.
var (
	updateMetaClient  = &http.Client{Timeout: 30 * time.Second}
	updateFetchClient = &http.Client{Timeout: 15 * time.Minute}
)

func updatePrint(s ...any) {
	fmt.Println(s...)
}

func isPinned() (pinned bool) {
	conf := CONFIG.Load()
	if conf.CLIConfig != nil {
		if conf.CLIConfig.PinVersion {
			return true
		}
	}
	return false
}

func skipUpdatePrompt() (pinned bool) {
	conf := CONFIG.Load()
	if conf.CLIConfig != nil {
		if conf.CLIConfig.SkipUpdatePrompt {
			return true
		}
	}
	return false
}

func doUpdate() {
	conf := CONFIG.Load()
	defer func() {
		if conf.UpdateCheckInterval == 0 {
			conf.UpdateCheckInterval = 1440
		}
		time.Sleep(time.Duration(conf.UpdateCheckInterval) * time.Minute)
	}()
	defer RecoverAndLog()

	if !conf.UpdateWhileConnected {
		isConnected := false
		tunnelMapRange(func(tun *TUN) bool {
			if tun.wgDevice != nil {
				isConnected = true
			}
			return true
		})
		if isConnected {
			return
		}
	}

	if conf.DisableUpdates {
		return
	}

	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return
	}

	isPinned := isPinned()
	var shouldUpdate bool

	var err error
	if isPinned {
		DEBUG("checking if client version is pinned")
		version, verr := getPinnedVersion()
		if verr != nil {
			return
		}
		DEBUG("downloading pinned version:", version)
		shouldUpdate, err = checkForAndDownloadUpdate(version)
	} else {
		DEBUG("downloading latest version")
		shouldUpdate, err = checkForAndDownloadUpdate("")
	}

	if err != nil {
		ERROR("update check/download failed:", err)
		return
	}
	if !shouldUpdate {
		return
	}

	DEBUG("launching update process")
	err = replaceCurrentVersion()
	if err != nil {
		// replaceCurrentVersion restores the previous binary itself when the
		// final rename fails; nothing more to roll back here.
		ERROR("error switching binaries during update:", err)
		return
	}

	INFO("update finished")
}

func doStartupUpdate() (didUpdate bool) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return false
	}

	conf := CONFIG.Load()
	if conf.DisableUpdates {
		return false
	}

	isPinned := isPinned()
	var err error
	var shouldUpdate bool

	updatePrint("Starting updater")
	if isPinned {
		version, verr := getPinnedVersion()
		if verr != nil {
			err = verr
		} else {
			updatePrint("Pinned version from server:", version)
			shouldUpdate, err = checkForAndDownloadUpdate(version)
		}
	} else {
		shouldUpdate, err = checkForAndDownloadUpdate("")
	}

	if !shouldUpdate {
		return false
	}
	if err != nil {
		updatePrint("Unable to update:", err)
		return false
	}

	if !skipUpdatePrompt() {
		shouldUpdate = yesNoPrompt("Update tunnels now ?")
	}
	if !shouldUpdate {
		return false
	}

	err = replaceCurrentVersion()
	if err != nil {
		updatePrint("Unable to replace current version with new tunnels version:", err)
		return false
	}
	updatePrint("update finished")
	return true
}

func checkForAndDownloadUpdate(targetTag string) (shouldUpdate bool, err error) {
	if targetTag != "" && targetTag == version.Version {
		return false, nil
	}

	url, tag, apiDigest, err := getReleaseInfo(targetTag)
	if err != nil {
		return false, err
	}

	versionNumber := strings.ReplaceAll(tag, "v", "")
	if versionNumber == version.Version {
		return false, nil
	}

	expectedSum, err := getExpectedChecksum(tag)
	if err != nil {
		return false, fmt.Errorf("unable to get expecetd sha sum from source: %s", err)
	}

	// Cross-check the checksums.txt entry against the GitHub API's asset
	// digest ("sha256:<hex>"). Not a signature, but both sources must agree —
	// a tampered checksums file alone no longer validates an archive.
	if hexSum, ok := strings.CutPrefix(apiDigest, "sha256:"); ok {
		if !strings.EqualFold(hexSum, expectedSum) {
			return false, fmt.Errorf("checksum mismatch between release digest (%s) and checksums.txt (%s)", hexSum, expectedSum)
		}
	}

	err = compareLocalArchiveToExpectedShaSum(expectedSum)
	if err != nil {
		assetResp, err := updateFetchClient.Get(url)
		if err != nil {
			return false, fmt.Errorf("failed to download asset: %w", err)
		}
		defer assetResp.Body.Close()

		if assetResp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("failed to download asset: received status code %d", assetResp.StatusCode)
		}

		state := STATE.Load()
		_ = os.Remove(state.BasePath + archive)
		out, err := os.OpenFile(state.BasePath+archive, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return false, fmt.Errorf("failed to create output file: %w", err)
		}
		defer out.Close()

		size, _ := strconv.ParseInt(assetResp.Header.Get("Content-Length"), 10, 64)
		progress := &ProgressWriter{Total: size, barWidth: 40}
		// Cap the download: a body larger than the cap is truncated and then
		// rejected by the checksum comparison below, instead of filling the disk.
		reader := io.TeeReader(io.LimitReader(assetResp.Body, maxUpdateArchiveSize), progress)
		_, err = io.Copy(out, reader)
		if err != nil {
			return false, fmt.Errorf("failed to write update to file: %w", err)
		}
		out.Sync()
	}

	err = compareLocalArchiveToExpectedShaSum(expectedSum)
	if err != nil {
		return false, err
	}

	return true, nil
}

func compareLocalArchiveToExpectedShaSum(remoteSum string) (err error) {
	state := STATE.Load()
	localShaSum, err := calculateSha256(state.BasePath + archive)
	if err != nil {
		return fmt.Errorf("unable to get expecetd sha sum from local file: %s", err)
	}

	if !strings.EqualFold(localShaSum, remoteSum) {
		_ = os.Remove(state.BasePath + archive)
		return fmt.Errorf("local binary hash invalid, expected (%s) got (%s)", remoteSum, localShaSum)
	}

	return nil
}

func replaceCurrentVersion() (err error) {
	conf := CONFIG.Load()

	var ex string
	ex, err = os.Executable()
	if err != nil || ex == "" {
		return fmt.Errorf("Error finding executable string(%s) err: %s", ex, err)
	}
	state := STATE.Load()
	_ = os.Remove(ex + nextVersionSuffix)
	err = untarGz(state.BasePath+archive, ex+nextVersionSuffix)
	if err != nil {
		return err
	}

	err = os.Rename(ex, ex+prevVersionSuffix)
	if err != nil {
		return err
	}

	err = os.Rename(ex+nextVersionSuffix, ex)
	if err != nil {
		err = os.Rename(ex+prevVersionSuffix, ex)
		if err != nil {
			return err
		}
		return err
	}

	if conf.RestartPostUpdate {
		argv0, _ := exec.LookPath(os.Args[0])
		err = syscall.Exec(argv0, os.Args, os.Environ())
		if err == nil {
			os.Exit(1)
		}

	} else if conf.ExitPostUpdate {
		os.Exit(1)
	}

	return
}

func calculateSha256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func getExpectedChecksum(tag string) (string, error) {
	url := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/tunnels_%s_checksums.txt", owner, repo, tag, strings.ReplaceAll(tag, "v", ""))
	resp, err := updateMetaClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	matching := fmt.Sprintf("tunnels_%s_%s_%s", strings.ReplaceAll(tag, "v", ""), runtime.GOOS, runtime.GOARCH)
	scanner := bufio.NewScanner(io.LimitReader(resp.Body, maxUpdateMetaSize))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), matching) {
			parts := strings.Fields(line)
			return parts[0], nil
		}
	}
	return "", errors.New("checksum not found for asset")
}

func getReleaseInfo(targetTag string) (url, tag, hash string, err error) {
	apiURL := ""
	if targetTag == "" {
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	} else {
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/v%s", owner, repo, targetTag)
	}

	resp, err := updateMetaClient.Get(apiURL)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxUpdateMetaSize))
	r := new(Release)
	err = json.Unmarshal(b, r)
	if err != nil {
		return "", "", "", err
	}

	for _, v := range r.Assets {
		if strings.Contains(v.BrowserDownloadURL, "server") {
			continue
		}
		if !strings.Contains(strings.ToLower(v.BrowserDownloadURL), strings.ToLower(runtime.GOOS)) {
			continue
		}
		if !strings.Contains(strings.ToLower(v.BrowserDownloadURL), strings.ToLower(runtime.GOARCH)) {
			continue
		}
		if r.Draft || r.Prerelease {
			continue
		}
		return v.BrowserDownloadURL, r.TagName, v.Digest, nil
	}

	return "", "", "", fmt.Errorf("no release found for os( %s ) arch( %s ) version( %s )", runtime.GOOS, runtime.GOARCH, targetTag)
}

type Release struct {
	URL       string `json:"url"`
	AssetsURL string `json:"assets_url"`
	UploadURL string `json:"upload_url"`
	HTMLURL   string `json:"html_url"`
	ID        int    `json:"id"`
	Author    struct {
		Login             string `json:"login"`
		ID                int    `json:"id"`
		NodeID            string `json:"node_id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		UserViewType      string `json:"user_view_type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"author"`
	NodeID          string    `json:"node_id"`
	TagName         string    `json:"tag_name"`
	TargetCommitish string    `json:"target_commitish"`
	Name            string    `json:"name"`
	Draft           bool      `json:"draft"`
	Immutable       bool      `json:"immutable"`
	Prerelease      bool      `json:"prerelease"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	PublishedAt     time.Time `json:"published_at"`
	Assets          []struct {
		URL      string `json:"url"`
		ID       int    `json:"id"`
		NodeID   string `json:"node_id"`
		Name     string `json:"name"`
		Label    string `json:"label"`
		Uploader struct {
			Login             string `json:"login"`
			ID                int    `json:"id"`
			NodeID            string `json:"node_id"`
			AvatarURL         string `json:"avatar_url"`
			GravatarID        string `json:"gravatar_id"`
			URL               string `json:"url"`
			HTMLURL           string `json:"html_url"`
			FollowersURL      string `json:"followers_url"`
			FollowingURL      string `json:"following_url"`
			GistsURL          string `json:"gists_url"`
			StarredURL        string `json:"starred_url"`
			SubscriptionsURL  string `json:"subscriptions_url"`
			OrganizationsURL  string `json:"organizations_url"`
			ReposURL          string `json:"repos_url"`
			EventsURL         string `json:"events_url"`
			ReceivedEventsURL string `json:"received_events_url"`
			Type              string `json:"type"`
			UserViewType      string `json:"user_view_type"`
			SiteAdmin         bool   `json:"site_admin"`
		} `json:"uploader"`
		ContentType        string    `json:"content_type"`
		State              string    `json:"state"`
		Size               int       `json:"size"`
		Digest             string    `json:"digest"`
		DownloadCount      int       `json:"download_count"`
		CreatedAt          time.Time `json:"created_at"`
		UpdatedAt          time.Time `json:"updated_at"`
		BrowserDownloadURL string    `json:"browser_download_url"`
	} `json:"assets"`
	TarballURL string `json:"tarball_url"`
	ZipballURL string `json:"zipball_url"`
	Body       string `json:"body"`
}

type ProgressWriter struct {
	Total    int64
	Written  int64
	full     bool
	barWidth int
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.Written += int64(n)
	pw.printProgress()
	return n, nil
}

func (pw *ProgressWriter) printProgress() {
	if pw.Total <= 0 {
		return
	}

	percentage := float64(pw.Written) / float64(pw.Total) * 100

	if percentage >= 100 && !pw.full {
		percentage = 99.9
	}

	filledWidth := int(float64(pw.barWidth) * (percentage / 100))

	bar := strings.Repeat("=", filledWidth) + ">" + strings.Repeat(" ", pw.barWidth-filledWidth)

	fmt.Printf("\rDownloading [%s] %.2f%% (%s / %s)", bar, percentage, formatBytes(pw.Written), formatBytes(pw.Total))

	if pw.Written >= pw.Total && !pw.full {
		pw.full = true
		bar = strings.Repeat("=", pw.barWidth+1)
		fmt.Printf("\rDownloading [%s] 100.00%% (%s / %s)\n", bar, formatBytes(pw.Written), formatBytes(pw.Total))
	}
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func untarGz(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)

unziploop:
	for {
		header, err := tr.Next()

		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}

		switch header.Typeflag {
		case tar.TypeReg:
			if !strings.Contains(strings.ToLower(header.Name), "tunnels") {
				continue unziploop
			}

			// Fixed 0755 — the archive-supplied mode is untrusted (it could
			// carry setuid/setgid bits).
			f, err := os.OpenFile(dest, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o755)
			if err != nil {
				return err
			}

			// Cap the decompressed size (gzip-bomb guard).
			if _, err := io.Copy(f, io.LimitReader(tr, maxUpdateArchiveSize)); err != nil {
				f.Close()
				return err
			}

			f.Close()
		}
	}
}

func getPinnedVersion() (version string, err error) {
	conf := CONFIG.Load()
	cliConf := conf.CLIConfig
	if cliConf == nil {
		return "", errors.New("no cli config")
	}

	var cs *ControlServer
	for i := range conf.ControlServers {
		if conf.ControlServers[i].ID == cliConf.ControlServerID {
			cs = conf.ControlServers[i]
		}
	}
	if cs == nil {
		return "", errors.New("no control server found")
	}

	resp, code, _ := SendRequestToURL(
		nil,
		"GET",
		cs.GetURL("/"),
		nil,
		5000,
		cs.ValidateCertificate,
	)

	if code != 200 {
		return "", errors.New("non 200 code from control server when checking client version")
	}

	hr := new(types.HealthResponse)
	err = json.Unmarshal(resp, hr)
	if err != nil {
		return "", errors.New("unable to decode health response when checking client pinned version")
	}

	return hr.ClientVersion, nil
}

func yesNoPrompt(label string) bool {
	var s string
	fmt.Printf("%s [y/n]: ", label)
	_, err := fmt.Scanln(&s)
	if err != nil {

		if err.Error() == "unexpected newline" {
			return false
		}
	}

	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	if s == "y" || s == "yes" {
		return true
	}
	return false
}
