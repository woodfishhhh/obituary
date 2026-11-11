package gui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/buildinfo"
	"github.com/woodfishhhh/obituary/internal/update"
)

func TestUpdateEmptyCacheAndPlanHaveNoNull(t *testing.T) {
	a, _ := fixture(t)
	info, err := a.Update()
	if err != nil || info.Current != buildinfo.Version || !info.Enabled || info.Channel != "other" || info.Newer {
		t.Fatalf("empty: %+v %v", info, err)
	}
	data, err := json.Marshal(info)
	if err != nil || strings.Contains(string(data), "null") || strings.Contains(string(data), "checkedAt") || strings.Contains(string(data), "published") {
		t.Fatalf("JSON: %s %v", data, err)
	}
	plan, err := a.PlanUpdateCheck(true)
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(plan)
	if err != nil || !strings.Contains(string(data), `"changes":[]`) {
		t.Fatalf("plan JSON: %s %v", data, err)
	}
}

func TestCheckUpdateManualWhileDisabledAndCacheOnlyRead(t *testing.T) {
	a, _ := fixture(t)
	write(t, filepath.Join(a.deps.ConfigDir, "config.toml"), "[update]\ncheck=false\n")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, `{"tag_name":"v9.0.0","name":"Release","body":"Release notes","html_url":"https://github.com/woodfishhhh/obituary/releases/tag/v9.0.0","published_at":"2026-11-11T00:00:00Z"}`)
	}))
	defer server.Close()
	a.deps.UpdateChecker = &update.Checker{BaseURL: server.URL}
	write(t, filepath.Join(filepath.Dir(a.deps.Executable), ".obituary-install.json"), `{}`)
	info, err := a.CheckUpdate()
	if err != nil || requests != 1 || !info.Newer || info.Enabled || info.Latest != "9.0.0" || info.Notes != "Release notes" || info.Name != "Release" || info.Published.IsZero() || info.CheckedAt.IsZero() || info.Channel != "script" || !strings.Contains(info.Command, "-Version 9.0.0") {
		t.Fatalf("manual: %+v %v", info, err)
	}
	cached, err := a.Update()
	if err != nil || requests != 1 || cached != info {
		t.Fatalf("read cache: %+v %v calls=%d", cached, err, requests)
	}
	server.Close()
	failed, err := a.CheckUpdate()
	if err != nil || failed.Error == "" || failed.Latest != info.Latest {
		t.Fatalf("error: %+v %v", failed, err)
	}
}

func TestCheckUpdateMirrorReleaseAndCommand(t *testing.T) {
	a, _ := fixture(t)
	github := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	github.Close()
	requests := 0
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, `{"tag_name":"v9.0.0","name":"Release","body":"Release notes"}`)
	}))
	defer mirror.Close()
	a.deps.UpdateChecker = &update.Checker{BaseURL: github.URL, MirrorURL: mirror.URL}
	write(t, filepath.Join(filepath.Dir(a.deps.Executable), ".obituary-install.json"), `{"owner":"obituary-install-v1","source":"mirror"}`)
	info, err := a.CheckUpdate()
	if err != nil || requests != 1 || info.Error != "" || !info.Newer || info.Latest != "9.0.0" || info.Channel != "script" {
		t.Fatalf("mirror: %+v %v", info, err)
	}
	if !strings.HasPrefix(info.Command, "irm https://cn.obituary.woodfish.site/install.ps1 ") || !strings.Contains(info.Command, "-Version 9.0.0") || !strings.HasSuffix(info.Command, " -Mirror cn") {
		t.Fatalf("mirror command: %s", info.Command)
	}
	cached, err := a.Update()
	if err != nil || requests != 1 || cached != info {
		t.Fatalf("mirror cache: %+v %v calls=%d", cached, err, requests)
	}
}

func TestCheckUpdateReportsSettingChangedDuringRequest(t *testing.T) {
	a, _ := fixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plan, err := a.PlanUpdateCheck(false)
		if err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return
		}
		if _, err := a.ApplyPlan(plan.ID); err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return
		}
		fmt.Fprint(w, `{"tag_name":"v9.0.0"}`)
	}))
	defer server.Close()
	a.deps.UpdateChecker = &update.Checker{BaseURL: server.URL}
	info, err := a.CheckUpdate()
	if err != nil || info.Enabled || info.Latest != "9.0.0" {
		t.Fatalf("concurrent setting: %+v %v", info, err)
	}
}

func TestPlanUpdateCheckPreservesConfigAndRejectsStalePlan(t *testing.T) {
	a, _ := fixture(t)
	path := filepath.Join(a.deps.ConfigDir, "config.toml")
	write(t, path, "# keep\n[graveyard]\nmode='recycle'\n[update]\ncheck=true # keep switch comment\n")
	plan, err := a.PlanUpdateCheck(false)
	if err != nil || len(plan.Changes) != 1 {
		t.Fatalf("plan: %+v %v", plan, err)
	}
	before, err := a.Update()
	if err != nil || !before.Enabled {
		t.Fatal("preview applied setting", err)
	}
	if _, err := a.ApplyPlan(plan.ID); err != nil {
		t.Fatal(err)
	}
	cfg, err := a.readConfig()
	if err != nil || cfg.Update.Check || cfg.Graveyard.Mode != "recycle" {
		t.Fatalf("config: %+v %v", cfg, err)
	}
	plan, err = a.PlanUpdateCheck(true)
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, "[update]\ncheck=false\n# edited after preview\n")
	if _, err := a.ApplyPlan(plan.ID); err == nil {
		t.Fatal("stale plan accepted")
	}
}
