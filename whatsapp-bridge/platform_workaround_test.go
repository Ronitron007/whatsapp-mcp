package main

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waWa6"
	"go.mau.fi/whatsmeow/store"
)

// WA rejects WEB-identified companions since 2026-06-09 (whatsmeow#1164).
// This pins the macOS identity workaround so a refactor can't silently
// revert the bridge to the blocked WEB platform.
func TestApplyPlatformWorkaround(t *testing.T) {
	applyPlatformWorkaround()
	if got := store.BaseClientPayload.UserAgent.GetPlatform(); got != waWa6.ClientPayload_UserAgent_MACOS {
		t.Errorf("UserAgent.Platform = %v, want MACOS", got)
	}
	if got := store.DeviceProps.GetPlatformType(); got != waCompanionReg.DeviceProps_CATALINA {
		t.Errorf("DeviceProps.PlatformType = %v, want CATALINA", got)
	}
	if got := store.DeviceProps.GetOs(); got != "Mac OS" {
		t.Errorf("DeviceProps.Os = %q, want \"Mac OS\"", got)
	}
}
