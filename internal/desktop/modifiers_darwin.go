//go:build darwin

package desktop

import (
	"strings"

	"golang.design/x/hotkey"
)

var modCmd = hotkey.ModCmd

var modAlt = hotkey.ModOption

// HumanReadableHotkey returns macOS symbol notation, e.g. "⌘⇧Space".
func HumanReadableHotkey(mods []hotkey.Modifier, key hotkey.Key) string {
	var parts []string
	for _, m := range mods {
		switch m {
		case modCmd:
			parts = append(parts, "⌘")
		case hotkey.ModCtrl:
			parts = append(parts, "⌃")
		case hotkey.ModShift:
			parts = append(parts, "⇧")
		case modAlt:
			parts = append(parts, "⌥")
		}
	}
	if name, ok := reverseKeyMap[key]; ok && len(name) > 0 {
		parts = append(parts, strings.ToUpper(name[:1])+name[1:])
	}
	return strings.Join(parts, "")
}
