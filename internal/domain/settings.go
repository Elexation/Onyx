package domain

import (
	"strconv"
	"time"
)

const (
	SettingTrashEnabled  = "trash.enabled"
	SettingTrashPurgeAge = "trash.purge_age"
	SettingTrashMaxSize  = "trash.max_size"
	SettingVersionsEnabled         = "versions.enabled"
	SettingVersionsMaxCount        = "versions.max_count"
	SettingVersionsMaxAge          = "versions.max_age"
	SettingVersionsMaxFileSize     = "versions.max_file_size"
	SettingVersionsMaxStorageBytes = "versions.max_storage"
	SettingSharesEnabled = "shares.enabled"
	SettingSharesDefaultExpiry = "shares.default_expiry"
	SettingSessionLifetime = "session.lifetime"
	SettingUploadMaxSize = "upload.max_size"
	SettingBrandingName = "branding.name"
)

var Defaults = map[string]string{
	SettingTrashEnabled:  "true",
	SettingTrashPurgeAge: "720h",       // 30 days
	SettingTrashMaxSize:  "0",          // 0 = unlimited, stored in bytes
	SettingVersionsEnabled:         "true",
	SettingVersionsMaxCount:        "10",
	SettingVersionsMaxAge:          "2160h",      // 90 days
	SettingVersionsMaxFileSize:     "1073741824", // 1 GiB
	SettingVersionsMaxStorageBytes: "0",          // 0 = unlimited, stored in bytes
	SettingSharesEnabled:       "true",
	SettingSharesDefaultExpiry: "168h",  // 7 days
	SettingSessionLifetime:     "720h",  // 30 days
	SettingUploadMaxSize:       "0",     // 0 = unlimited
	SettingBrandingName:        "Onyx",
}

func GetBool(value string) bool {
	return value == "true" || value == "1"
}

func GetInt(value string) int {
	n, _ := strconv.Atoi(value)
	return n
}

func GetInt64(value string) int64 {
	n, _ := strconv.ParseInt(value, 10, 64)
	return n
}

func GetDuration(value string) time.Duration {
	d, _ := time.ParseDuration(value)
	return d
}
