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
		content, err = hermesParentState(registration.orig, container, kind)
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
		registration, err = removeHermesParents(registration, content, container, kind)
		if err != nil {
			return nil, err
		}
	}
	changes, err := OrderManagedInstallation(assets, []Change{registration}, uninstall)
	if err != nil {
		return nil, err
	}
	// Preserve the MCP planner's primary-file-first result for inspectors.
	return append(changes[len(changes)-1:], changes[:len(changes)-1]...), nil
}

func hermesParentState(original []byte, container, kind string) ([]byte, error) {
	data, err := configJSON(original, ConfigYAML)
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
	return marshal(state)
}

func removeHermesParents(registration Change, content []byte, container, kind string) (Change, error) {
	var state hermesConfigParents
	if err := json.Unmarshal(content, &state); err != nil {
		return registration, err
	}
	data := registration.orig
	if registration.data != nil {
		data = registration.data
	}
	changed := false
	var err error
	if kind == "hooks" && !state.EnabledExisted {
		data, changed, err = removeEmptyHermesMember(data, []string{container, "enabled"}, "[]")
		if err != nil {
			return registration, err
		}
	}
	if !state.ContainerExisted {
		var removed bool
		data, removed, err = removeEmptyHermesMember(data, []string{container}, "{}")
		if err != nil {
			return registration, err
		}
		changed = changed || removed
	}
	if changed {
		registration = setIntegrationData(registration, data)
	}
	return registration, nil
}

func removeEmptyHermesMember(data []byte, path []string, empty string) ([]byte, bool, error) {
	raw, err := configJSON(data, ConfigYAML)
	if err != nil {
		return nil, false, err
	}
	member, err := configMember(raw, path)
	if err != nil {
		return nil, false, err
	}
	if sameConfigJSON(member, []byte(empty)) {
		edit := ConfigEdit{Path: path, Remove: true}
		data, err = editConfig(data, ConfigYAML, []ConfigEdit{edit})
		if err != nil {
			return nil, false, err
		}
		return data, true, nil
	}
	return data, false, nil
}
