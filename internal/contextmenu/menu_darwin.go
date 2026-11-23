package contextmenu

import (
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"howett.net/plist"
	"path/filepath"
)

// Finder discovers the service from Info.plist and Automator executes the Run Shell Script action.
func nativeMenus(exe string, lang i18n.Language) (map[string]Tree, error) {
	home, err := userdirs.Home()
	if err != nil {
		return nil, err
	}
	files := map[string]Tree{}
	type dict = map[string]any
	for _, a := range menuActions {
		label := a.label(lang)
		dir := filepath.Join(home, "Library/Services", label+".workflow", "Contents")
		service := dict{"NSMenuItem": dict{"default": label}, "NSMessage": "runWorkflowAsService", "NSSendFileTypes": []string{"public.item"}, "NSRequiredContext": dict{"NSApplicationIdentifier": "com.apple.finder"}}
		if a.id == "policy" {
			service["NSSendFileTypes"] = []string{"public.folder"}
		}
		params := dict{"COMMAND_STRING": a.command(shellQuote(exe), "\"$@\""), "inputMethod": 1, "shell": "/bin/sh", "source": ""}
		inputType := "com.apple.Automator.fileSystemObject"
		if a.id == "policy" {
			inputType = "com.apple.Automator.folder"
		}
		workflow := dict{"AMApplicationBuild": "523", "AMApplicationVersion": "2.10", "AMDocumentVersion": "2", "actions": []dict{{"action": dict{"AMActionVersion": "2.0.3", "AMParameterProperties": dict{}, "ActionBundlePath": "/System/Library/Automator/Run Shell Script.action", "ActionName": "Run Shell Script", "ActionParameters": params, "BundleIdentifier": "com.apple.RunShellScript", "Class Name": "RunShellScriptAction", "isViewVisible": true}, "isViewVisible": true}}, "connectors": dict{}, "workflowMetaData": dict{"workflowTypeIdentifier": "com.apple.Automator.servicesMenu", "serviceInputTypeIdentifier": inputType, "serviceOutputTypeIdentifier": "com.apple.Automator.nothing", "serviceApplicationBundleID": "com.apple.finder", "serviceApplicationPath": "/System/Library/CoreServices/Finder.app", "serviceProcessesInput": 0}}
		action := workflow["actions"].([]dict)[0]["action"].(dict)
		action["AMAccepts"] = dict{"Container": "List", "Optional": true, "Types": []string{"com.apple.cocoa.string"}}
		action["AMProvides"] = dict{"Container": "List", "Types": []string{"com.apple.cocoa.string"}}
		action["UUID"] = "307857C3-7E1D-48F0-AD3C-F98A288923E1"
		action["InputUUID"] = "09072222-443B-473E-9576-905F914E0278"
		action["OutputUUID"] = "37B9677B-EF7D-410A-BE64-C6808E55FBEC"
		for name, obj := range map[string]dict{"Info.plist": {"NSServices": []dict{service}}, "document.wflow": workflow} {
			data, err := plist.Marshal(obj, plist.XMLFormat)
			if err != nil {
				return nil, err
			}
			files[filepath.Join(dir, name)] = menuFile(string(data), 0600)
		}
	}
	return files, nil
}
