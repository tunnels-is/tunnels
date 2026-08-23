//go:build windows

package client

import (
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// validateUserKeyFile checks the Windows DACL. Go's FileMode on Windows is
// only 0666 (writable) or 0444 (read-only), so a Unix 0600 test always
// false-positives on files we just created — including the second launch
// after an elevated first run.
func validateUserKeyFile(path string, _ os.FileInfo) error {
	return validateWindowsSecretACL(path)
}

func secureSecretFile(path string) error {
	sid, err := currentUserSID()
	if err != nil {
		return err
	}
	sddl := "D:P(A;;FA;;;" + sid.String() + ")(A;;FA;;;SY)(A;;FA;;;BA)"
	return applyProtectedDACL(path, sddl)
}

func currentUserSID() (*windows.SID, error) {
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	if tu.User.Sid == nil {
		return nil, fmt.Errorf("token has no user SID")
	}
	sid, err := tu.User.Sid.Copy()
	if err != nil {
		return nil, err
	}
	if sid.String() == "" {
		return nil, fmt.Errorf("cannot stringify user SID")
	}
	return sid, nil
}

func applyProtectedDACL(path, sddl string) error {
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return fmt.Errorf("parse secret-file SDDL: %w", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("secret-file DACL: %w", err)
	}
	if dacl == nil {
		return fmt.Errorf("secret-file SDDL produced a NULL DACL")
	}
	err = windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	)
	runtime.KeepAlive(sd)
	return err
}

func validateWindowsSecretACL(path string) error {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("read DACL: %w", err)
	}
	defer runtime.KeepAlive(sd)

	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("no DACL: %w", err)
	}
	if dacl == nil {
		return fmt.Errorf("NULL DACL (world-accessible)")
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return fmt.Errorf("read ACE %d: %w", i, err)
		}
		if ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
			continue
		}
		if ace.Mask == 0 {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if sidGrantsWorldAccess(sid) {
			return fmt.Errorf("DACL grants access to %s", formatSID(sid))
		}
	}
	return nil
}

func sidGrantsWorldAccess(sid *windows.SID) bool {
	if sid == nil || !sid.IsValid() {
		return false
	}
	switch {
	case sid.IsWellKnown(windows.WinWorldSid),
		sid.IsWellKnown(windows.WinBuiltinUsersSid),
		sid.IsWellKnown(windows.WinAuthenticatedUserSid),
		sid.IsWellKnown(windows.WinBuiltinGuestsSid),
		sid.IsWellKnown(windows.WinAnonymousSid),
		sid.IsWellKnown(windows.WinInteractiveSid),
		sid.IsWellKnown(windows.WinAccountDomainUsersSid):
		return true
	default:
		return false
	}
}

func formatSID(sid *windows.SID) string {
	s := sid.String()
	name, domain, _, err := sid.LookupAccount("")
	if err != nil || name == "" {
		return s
	}
	if domain != "" {
		return domain + `\` + name + " (" + s + ")"
	}
	return name + " (" + s + ")"
}
