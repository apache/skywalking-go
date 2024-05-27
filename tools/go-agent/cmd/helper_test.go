package main

import "testing"

func TestVersion(t *testing.T) {
	version = "0.4.0"
	goVersion = "1.22"
	gitCommit = "f417435"

	PrintVersion()
}
