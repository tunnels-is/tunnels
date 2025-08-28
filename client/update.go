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
	repo              = "tunnels"
	owner             = "tunnels-is"
)

// Used to wrap Println to avoid false detection
// during build process
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
	defer func() {
		conf := CONFIG.Load()
		// never allow 0 update interval
		if conf.UpdateCheckInterval == 0 {
			conf.UpdateCheckInterval = 1440
		}
		time.Sleep(time.Duration(conf.UpdateCheckInterval) * time.Minute)
	}()
	defer RecoverAndLog()

	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return
	}
	isPinned := isPinned()

	var err error
	if isPinned {
		DEBUG("checking if client version is pinned")
		version, verr := getPinnedVersion()
		if verr != nil {
			return
		}
		DEBUG("downloading pinned version:", version)
		err = downloadUpdate(version)
	} else {
		DEBUG("downloading latest version")
		err = downloadUpdate("")
	}

	if err == nil {
		DEBUG("performing in-place update")
		err = replaceCurrentVersion()
		if err != nil {
			ERROR("error while performing in-place update:", err)
		}
		err = cleanupUpdateFiles()
		if err != nil {
			ERROR("error while cleaning up post-update", err)
		}
		return
	}
}

func doStartupUpdate() {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return
	}

	isPinned := isPinned()
	var err error

	updatePrint("Downloading update..")
	if isPinned {
		version, verr := getPinnedVersion()
		if verr != nil {
			err = verr
		} else {
			updatePrint("Pinned version from server:", version)
			err = downloadUpdate(version)
		}
	} else {
		err = downloadUpdate("")
	}

	var shouldUpdate bool
	if err != nil && !skipUpdatePrompt() {
		shouldUpdate = yesNoPrompt("Update tunnels now ?")
	}

	if err == nil && shouldUpdate {
		err = replaceCurrentVersion()
		if err != nil {
			updatePrint("Unable to replace current version with new tunnels version:", err)
		}
		err = cleanupUpdateFiles()
		if err != nil {
			updatePrint("cleaning up files post update", err)
		}
		return
	}

	updatePrint("Unable to update:", err)
}

func downloadUpdate(targetTag string) error {
	if targetTag != "" && targetTag == version.Version {
		return nil
	}

	url, tag, _, err := getReleaseInfo(targetTag)
	if err != nil {
		return err
	}

	versionNumber := strings.ReplaceAll(tag, "v", "")
	if versionNumber == version.Version {
		return nil
	}

	expectedSum, err := getExpectedChecksum(tag)
	if err != nil {
		return fmt.Errorf("unable to get expecetd sha sum from source: %s", err)
	}

	err = compareLocalArchiveToExpectedShaSum(expectedSum)
	if err != nil {
		assetResp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to download asset: %w", err)
		}
		defer assetResp.Body.Close()

		if assetResp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to download asset: received status code %d", assetResp.StatusCode)
		}

		state := STATE.Load()
		_ = os.Remove(state.BasePath + archive)
		out, err := os.Create(state.BasePath + archive)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer out.Close()

		size, _ := strconv.ParseInt(assetResp.Header.Get("Content-Length"), 10, 64)
		progress := &ProgressWriter{Total: size, barWidth: 40}
		reader := io.TeeReader(assetResp.Body, progress)
		_, err = io.Copy(out, reader)
		if err != nil {
			return fmt.Errorf("failed to write update to file: %w", err)
		}
		out.Sync()
	}

	err = compareLocalArchiveToExpectedShaSum(expectedSum)
	if err != nil {
		return err
	}

	return nil
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

func cleanupUpdateFiles() (err error) {
	ex, err := os.Executable()
	if err != nil {
		return err
	}
	state := STATE.Load()
	_ = os.Remove(state.BasePath + archive)
	_ = os.Remove(state.BasePath + ex + ".prev")
	return nil
}

func replaceCurrentVersion() (err error) {
	conf := CONFIG.Load()
	if !conf.ExitPostUpdate && !conf.RestartPostUpdate {
		return
	}

	ex, err := os.Executable()
	if err != nil {
		return err
	}
	state := STATE.Load()
	_ = os.Remove(ex + nextVersionSuffix)
	if runtime.GOOS == "windows" {
		// TODO...
	} else {
		err = untarGz(state.BasePath+archive, ex+nextVersionSuffix)
	}
	if err != nil {
		return err
	}

	err = os.Rename(ex, ex+".prev")
	if err != nil {
		return err
	}

	err = os.Rename(ex+nextVersionSuffix, ex)
	if err != nil {
		err = os.Rename(ex+".prev", ex)
		if err != nil {
			return err
		}
		return err
	}

	if conf.RestartPostUpdate {
		fmt.Println("Post update restart..")
		argv0, _ := exec.LookPath(os.Args[0])
		syscall.Exec(argv0, os.Args, os.Environ())
		os.Exit(1)
	} else if conf.ExitPostUpdate {
		fmt.Println("Update finished, exiting..")
		os.Exit(1)
	}

	return nil
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
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	matching := fmt.Sprintf("tunnels_%s_%s_%s", strings.ReplaceAll(tag, "v", ""), runtime.GOOS, runtime.GOARCH)
	scanner := bufio.NewScanner(resp.Body)
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

	resp, err := http.Get(apiURL)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
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
		if time.Since(r.PublishedAt).Hours() < 24 {
			continue
		}
		return v.BrowserDownloadURL, r.TagName, v.Digest, nil
	}

	return "", "", "", fmt.Errorf("no release found for os( %s ) arch( %s ) version( %s )", runtime.GOOS, runtime.GOARCH, tag)
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

// ProgressWriter is a custom writer to track download progress.
type ProgressWriter struct {
	Total    int64
	Written  int64
	full     bool // To ensure the 100% line is printed only once
	barWidth int
}

// Write implements the io.Writer interface.
func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.Written += int64(n)
	pw.printProgress()
	return n, nil
}

// printProgress displays the progress bar.
func (pw *ProgressWriter) printProgress() {
	if pw.Total <= 0 { // Don't display if total size is unknown
		return
	}

	percentage := float64(pw.Written) / float64(pw.Total) * 100

	// Prevent printing 100% until it's actually done
	if percentage >= 100 && !pw.full {
		percentage = 99.9
	}

	filledWidth := int(float64(pw.barWidth) * (percentage / 100))

	bar := strings.Repeat("=", filledWidth) + ">" + strings.Repeat(" ", pw.barWidth-filledWidth)

	// Use carriage return '\r' to stay on the same line
	fmt.Printf("\rDownloading [%s] %.2f%% (%s / %s)", bar, percentage, formatBytes(pw.Written), formatBytes(pw.Total))

	if pw.Written >= pw.Total && !pw.full {
		pw.full = true
		bar = strings.Repeat("=", pw.barWidth+1)
		fmt.Printf("\rDownloading [%s] 100.00%% (%s / %s)\n", bar, formatBytes(pw.Written), formatBytes(pw.Total))
	}
}

// formatBytes is a helper to format bytes into KB, MB, etc.
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
			fmt.Println("ZIP:", header.Name)
			if !strings.Contains(strings.ToLower(header.Name), "tunnels") {
				continue unziploop
			}

			f, err := os.OpenFile(dest, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(f, tr); err != nil {
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

	resp, code, err := SendRequestToURL(
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
		// If nothing is entered, it's considered a "no"
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
