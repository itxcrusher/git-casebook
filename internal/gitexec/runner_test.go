package gitexec

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOfflineRunnerRejectsNetworkAndMutationCommands(t *testing.T) {
	runner, err := New(filepath.Join(t.TempDir(), "control"), Limits{CommandTimeout: 10 * time.Second, AcquisitionTimeout: 10 * time.Second, MaxOutputBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"clone", "fetch", "pull", "push", "ls-remote", "submodule", "checkout", "reset"} {
		if _, err := runner.Run(context.Background(), ClassAnalysis, "", command); err == nil {
			t.Fatalf("offline runner accepted %s", command)
		}
	}
	if len(runner.Records()) != 0 {
		t.Fatal("rejected commands must not execute or enter the command audit")
	}
}

func TestControlledEnvironmentProbesGit(t *testing.T) {
	runner, err := New(filepath.Join(t.TempDir(), "control"), Limits{CommandTimeout: 10 * time.Second, AcquisitionTimeout: 10 * time.Second, MaxOutputBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	version, err := runner.Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if version == "" {
		t.Fatal("git version was empty")
	}
	if records := runner.Records(); len(records) != 1 || records[0].NetworkCapable {
		t.Fatalf("unexpected command audit: %+v", records)
	}
}
