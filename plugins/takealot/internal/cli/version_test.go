package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	previous := options
	defer func() { options = previous }()
	options = rootOptions{}

	var output bytes.Buffer
	command := newRootCommand()
	command.SetArgs([]string{"version"})
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatalf("execute version: %v", err)
	}

	want := "takealot " + Version + "\n"
	if output.String() != want {
		t.Fatalf("version output = %q, want %q", output.String(), want)
	}
}

func TestVersionCommandJSON(t *testing.T) {
	previous := options
	defer func() { options = previous }()
	options = rootOptions{}

	var output bytes.Buffer
	command := newRootCommand()
	command.SetArgs([]string{"version", "--json"})
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatalf("execute version: %v", err)
	}

	var got versionOutput
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("decode version JSON: %v", err)
	}
	want := versionOutput{Version: Version, Commit: Commit, BuildDate: BuildDate}
	if got != want {
		t.Fatalf("version JSON = %+v, want %+v", got, want)
	}
}

func TestVersionFlag(t *testing.T) {
	var output bytes.Buffer
	command := newRootCommand()
	command.SetArgs([]string{"--version"})
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatalf("execute version flag: %v", err)
	}
	if output.Len() == 0 {
		t.Fatal("version flag produced no output")
	}
}
