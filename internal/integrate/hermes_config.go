package integrate

import "encoding/json"

// Only remember whether these fixed containers existed. The existing managed
// asset protocol protects the receipt; no user YAML or secrets are copied.
type hermesConfigParents struct {
	ContainerExisted bool `json:"container_existed"`
	EnabledExisted   bool `json:"enabled_existed"`
}

func planHermesConfigParents(s Scope, registration Change, kind string, uninstall bool) ([]Change, error) {
	path := registration.Path + ".obituary-hermes-" + kind
	asset := ManagedAsset{Path: path, ID: "hermes-config-" + kind, Owner: "hermes", Version: "1"}
	p, err := readSharedAsset(s, asset)
	if err != nil {
		return nil, err
	}
	before := p.file
	container := "mcp_servers"
	if kind == "hooks" {
		container = "plugins"
	}
	content := before.orig
	if !before.existed {
		data, err := configJSON(registration.orig, ConfigYAML)
		if err != nil {
			return nil, err
		}
		parent, err := configMember(data, []string{container})
		if err != nil {
			return nil, err
		}
		state := hermesConfigParents{ContainerExisted: parent != nil}
		if kind == "hooks" {
			member, err := configMember(data, []string{container, "enabled"})
			if err != nil {
				return nil, err
			}
			state.EnabledExisted = member != nil
		}
		content, err = marshal(state)
		if err != nil {
			return nil, err
		}
	}
	asset.Content = content
	assets, err := planSharedRegistration(s, asset, nil, []string{"hermes"}, uninstall)
	if err != nil {
		return nil, err
	}
	if uninstall && before.existed && PreservedError(assets) == nil {
		var state hermesConfigParents
		if err := json.Unmarshal(content, &state); err != nil {
			return nil, err
		}
		data := registration.orig
		if registration.data != nil {
			data = registration.data
		}
		changed := false
		removeEmpty := func(path []string, empty string) error {
			raw, err := configJSON(data, ConfigYAML)
			if err != nil {
				return err
			}
			member, err := configMember(raw, path)
			if err != nil {
				return err
			}
			if sameConfigJSON(member, []byte(empty)) {
				edit := ConfigEdit{Path: path, Remove: true}
				data, err = editConfig(data, ConfigYAML, []ConfigEdit{edit})
				if err != nil {
					return err
				}
				changed = true
			}
			return nil
		}
		if kind == "hooks" && !state.EnabledExisted {
			if err := removeEmpty([]string{container, "enabled"}, "[]"); err != nil {
				return nil, err
			}
		}
		if !state.ContainerExisted {
			if err := removeEmpty([]string{container}, "{}"); err != nil {
				return nil, err
			}
		}
		if changed {
			registration = setIntegrationData(registration, data)
		}
	}
	changes, err := OrderManagedInstallation(assets, []Change{registration}, uninstall)
	if err != nil {
		return nil, err
	}
	// Preserve the MCP planner's primary-file-first result for inspectors.
	return append(changes[len(changes)-1:], changes[:len(changes)-1]...), nil
}
