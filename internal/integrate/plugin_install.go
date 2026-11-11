package integrate

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// These are self-contained modules. Inlining the transport avoids additional
// auto-discovered plugin entrypoints. No third-party plugin source is copied.
//
//go:embed plugins/*/*.ts plugins/openclaw/*.json
var pluginFiles embed.FS

func planPlugin(s Scope, dir, agent, id, template, destination, executable string, uninstall bool) ([]Change, error) {
	path := filepath.Join(dir, destination)
	settingsPath := filepath.Join(filepath.Dir(path), "obituary", agent+".json")
	if agent == "openclaw" {
		settingsPath = filepath.Join(filepath.Dir(path), "runtime.json")
	}
	for _, p := range []string{path, settingsPath} {
		if err := s.checkInside(filepath.Dir(dir), p, ManagedAssetReceiptPath(p), p+".bak"); err != nil {
			return nil, err
		}
	}
	settings, err := readIntegrationChange(s, settingsPath)
	if err != nil {
		return nil, err
	}
	var runtime struct {
		Executable string `json:"executable"`
		GuardRM    bool   `json:"guardRM"`
	}
	if settings.existed && json.Unmarshal(settings.orig, &runtime) != nil {
		if !uninstall {
			return nil, fmt.Errorf("%s: invalid plugin runtime settings", settingsPath)
		}
	}
	runtime.Executable, runtime.GuardRM = executable, runtime.GuardRM || s.GuardRM
	if !uninstall && (!filepath.IsAbs(executable) || !isObitMCP(executable, []string{"mcp"})) {
		return nil, fmt.Errorf("plugin needs an absolute obit.exe path")
	}
	config, err := marshal(runtime)
	if err != nil {
		return nil, err
	}
	payload, err := pluginFiles.ReadFile(template)
	if err != nil {
		return nil, err
	}
	bridge, err := pluginFiles.ReadFile("plugins/bridge/transport.ts")
	if err != nil {
		return nil, err
	}
	payload = bytes.ReplaceAll(payload, []byte("/* OBIT_BRIDGE */"), bridge)
	// V1 loaders can treat every named function export as another plugin.
	// Keep the installed surface to the documented default entrypoint.
	payload = bytes.ReplaceAll(payload, []byte("export function "), []byte("function "))
	changes, err := PlanManagedAssets(s, []ManagedAsset{
		{Path: settingsPath, ID: agent + "-runtime", Owner: agent, Version: "1", Content: append(config, '\n')},
		{Path: path, ID: id, Owner: agent, Version: "1", Content: payload, Activation: true},
	}, uninstall)
	if err != nil {
		return nil, err
	}
	for i := range changes {
		if changes[i].Path == path && changes[i].Action != "preserve" && !uninstall {
			prefix := "OBIT_" + strings.ToUpper(agent) + "_"
			changes[i].Detail = "after verifying all observed tools use the local Windows filesystem, launch the host with " + prefix + "LOCAL=1; guard also requires " + prefix + "SHELL=bash or powershell matching the actual shell; unknown/remote backends remain skipped"
			if agent == "openclaw" {
				changes[i].Detail += "; set OBIT_OPENCLAW_CWD to the verified workspace when hook context has no directory"
			}
		}
	}
	return changes, nil
}

// Each host has a distinct block even in a shared AGENTS.md. Outside bytes,
// including a BOM, CRLF and the user's comments, survive install and uninstall.
func planPluginRules(s Scope, path, agent string, uninstall bool) (Change, error) {
	if err := s.checkInside(filepath.Dir(path), path, path+".bak"); err != nil {
		return Change{}, err
	}
	ch, err := readIntegrationChange(s, path)
	if err != nil {
		return ch, err
	}
	begin := "<!-- obit:" + agent + ":begin -->"
	end := "<!-- obit:" + agent + ":end -->"
	text := string(ch.orig)
	start, finish, err := markdownMarkedRange(text, begin, end)
	if err != nil {
		return ch, fmt.Errorf("%s: %w", path, err)
	}
	eol := "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	content := strings.ReplaceAll(begin+"\n"+strings.TrimSpace(Awareness)+"\n"+end, "\n", eol)
	if start >= 0 {
		if uninstall {
			content = ""
		}
		return setIntegrationData(ch, []byte(text[:start]+content+text[finish:])), nil
	}
	if uninstall {
		return ch, nil
	}
	separator := ""
	if len(text) > 0 && text != "\xef\xbb\xbf" && !strings.HasSuffix(text, "\n") {
		separator = eol
	}
	return setIntegrationData(ch, []byte(text+separator+content+eol)), nil
}

func planPluginMCP(s Scope, path, agent, executable string, uninstall bool) ([]Change, error) {
	ownerPath := path + ".obituary-owners"
	for _, p := range []string{path, ownerPath} {
		if err := s.checkInside(filepath.Dir(path), p, p+".bak", ManagedAssetReceiptPath(p)); err != nil {
			return nil, err
		}
	}
	return planSharedMCP(s, path, agent, MCPConfig{Format: ConfigJSON, ServersPath: []string{"mcpServers"}, Codec: CommandArgsMCP{}}, executable, uninstall)
}
