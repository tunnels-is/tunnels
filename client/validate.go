package client

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ValidationError is a set of field-level problems (tunnel save, etc.).
type ValidationError struct {
	Messages []string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ""
	}
	return strings.Join(e.Messages, "; ")
}

var azCharCheck = regexp.MustCompile(`^[a-zA-Z0-9]*$`)

var tagCharCheck = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

var allowedConfigFormats = map[string]struct{}{
	"": {}, ".json": {}, ".conf": {}, ".yaml": {}, ".yml": {},
}

// safeTunnelTag validates tunnel tags used as on-disk identifiers.
func safeTunnelTag(tag string) bool {
	return tagCharCheck.MatchString(tag)
}

// safeListTag validates DNS blocklist/whitelist tags used as filenames under
// blocklists/ and whitelists/. Same charset as tunnel tags to prevent path traversal.
func safeListTag(tag string) bool {
	return tagCharCheck.MatchString(tag)
}

// listFilePath joins baseDir with a validated, lowercased list tag and ensures
// the result stays inside baseDir (no ".." / separator escapes).
func listFilePath(baseDir, tag string) (string, error) {
	if !safeListTag(tag) {
		return "", fmt.Errorf("invalid list tag %q: only a-z A-Z 0-9 _ - allowed", tag)
	}
	lower := strings.ToLower(tag)
	base := filepath.Clean(baseDir)
	path := filepath.Join(base, lower)
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("list path escapes base directory")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("list path escapes base directory")
	}
	return path, nil
}

func validateTunnelMeta(tun *TunnelMeta, oldTag string) (err []string) {
	ifnamemap := make(map[string]struct{})
	ifFail := azCharCheck.MatchString(tun.IFName)
	if !ifFail {
		err = append(err, "tunnel names can only contain a-z A-Z 0-9, invalid name: "+tun.IFName)
	}

	if !safeTunnelTag(tun.Tag) {
		err = append(err, "tunnel tag may only contain a-z A-Z 0-9 _ - , invalid tag: "+tun.Tag)
	}
	if oldTag != "" && !safeTunnelTag(oldTag) {
		err = append(err, "invalid old tunnel tag: "+oldTag)
	}
	if _, ok := allowedConfigFormats[tun.ConfigFormat]; !ok {
		err = append(err, "unsupported tunnel config format: "+tun.ConfigFormat)
	}

	tunnelMetaMapRange(func(t *TunnelMeta) bool {
		if t.Tag == tun.Tag {
			return true
		}
		ifnamemap[strings.ToLower(t.IFName)] = struct{}{}
		return true
	})

	_, ok := ifnamemap[strings.ToLower(tun.IFName)]
	if ok {
		if strings.ToLower(tun.IFName) != oldTag {
			err = append(err,
				"you cannot have two tunnels with the same interface name: "+tun.IFName,
			)
		}
	}

	if len(tun.IFName) < 3 {
		err = append(err, fmt.Sprintf("tunnel name should not be less then 3 characters (%s)", tun.IFName))
	}

	errx := ValidateAdapterID(tun)
	if errx != nil {
		err = append(err, errx.Error())
	}

	for _, h := range tun.AllowedHosts {
		if _, errp := NormalizeAllowedHost(h); errp != nil {
			err = append(err, errp.Error())
		}
	}

	return
}
