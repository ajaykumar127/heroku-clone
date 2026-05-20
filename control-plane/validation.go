package main

import (
	"fmt"
	"regexp"
)

var (
	appNameRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,38}[a-z0-9]$|^[a-z0-9]$`)
	commitSHARe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	configKeyRe = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,255}$`)
)

func validateAppName(name string) error {
	if !appNameRe.MatchString(name) {
		return fmt.Errorf("app name must be 1-40 lowercase alphanumeric characters or hyphens, start and end with alphanumeric")
	}
	return nil
}

func validateCommitSHA(commit string) error {
	if !commitSHARe.MatchString(commit) {
		return fmt.Errorf("commit must be a 7-40 character hex SHA")
	}
	return nil
}

func validateConfigKey(key string) error {
	if !configKeyRe.MatchString(key) {
		return fmt.Errorf("config key must be uppercase letters, digits, underscores, max 256 chars")
	}
	return nil
}

func validateConfigValue(value string) error {
	if len(value) > 65536 {
		return fmt.Errorf("config value exceeds maximum length of 65536 bytes")
	}
	return nil
}
