package doctor

import (
	"encoding/json"
	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/docker"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDoctorVolumeRecovery(t *testing.T) {
	_, exe := doctorAgentFixture(t)
	home := os.Getenv("OBIT_HOME")
	root := filepath.Join(home, "graveyard", "docker-volumes")
	for i, phase := range []string{"", "events-incomplete", "removed"} {
		name := []string{"cache-old", "cache-events", "cache-commit"}[i]
		archive := filepath.Join(root, []string{"1-aaaaaaaaaaaaaaaa", "2-bbbbbbbbbbbbbbbb", "3-cccccccccccccccc"}[i], "volume.tar")
		v := docker.Volume{Name: name, Driver: "local", Scope: "local", CreatedAt: time.Now(), Target: docker.Target{Endpoint: "unix:///test", DaemonID: "test", OSType: "linux"}}
		m := struct {
			Version int `json:"version"`
			ledger.Burial
			Phase string `json:"volumePhase,omitempty"`
		}{1, ledger.Burial{Kind: ledger.KindDockerVolume, ID: int64(i + 1), Path: name, Volume: &v, Grave: archive, VolumeArchiveDigest: strings.Repeat("a", 64)}, phase}
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		clitest.WriteFixture(t, archive, "archive")
		clitest.WriteFixture(t, archive+".json", string(data))
	}
	clitest.WriteFixture(t, filepath.Join(root, "4-dddddddddddddddd", "volume.tar.partial"), "incomplete")
	out, _, _ := clitest.Execute(t, newCmdWithAgents(exe, []string{"claude"}), "--json")
	var checks []DoctorCheck
	if err := json.Unmarshal([]byte(out), &checks); err != nil {
		t.Fatal(err, out)
	}
	var warnings []string
	for _, c := range checks {
		if c.Name == "docker-volume-recovery" && c.Status == "WARN" {
			warnings = append(warnings, c.Detail)
		}
	}
	if len(warnings) != 4 {
		t.Fatalf("volume recovery warnings: %v", warnings)
	}
	for i, want := range []string{"removal unconfirmed", "event evidence incomplete", "ledger commit unconfirmed", "archive incomplete"} {
		if !strings.Contains(warnings[i], want) {
			t.Errorf("missing %q in %s", want, warnings[i])
		}
	}
	for i, name := range []string{"cache-old", "cache-events", "cache-commit"} {
		if !strings.Contains(warnings[i], "docker volume inspect -- "+name) {
			t.Error(warnings[i])
		}
	}
}

func TestDoctorCommittedVolumeIsQuiet(t *testing.T) {
	home := t.TempDir()
	archive := filepath.Join(home, "graveyard", "docker-volumes", "1-aaaaaaaaaaaaaaaa", "volume.tar")
	st := ledger.State{Burials: []ledger.Burial{{Kind: ledger.KindDockerVolume, ID: 1, Grave: archive}}}
	clitest.WriteFixture(t, archive, "archive")
	if rows := english.diagnoseVolumeRecovery(home, st); len(rows) != 0 {
		t.Fatal(rows)
	}
	clitest.WriteFixture(t, archive+".json", "left after commit")
	if rows := english.diagnoseVolumeRecovery(home, st); len(rows) != 0 {
		t.Fatal(rows)
	}
}
