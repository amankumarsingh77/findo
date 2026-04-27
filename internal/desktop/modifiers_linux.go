//go:build linux

package desktop

import (
	"strings"

	"golang.design/x/hotkey"
)

var modCmd = hotkey.Mod4

var modAlt = hotkey.Mod1

// HumanReadableHotkey formats a modifier+key combination as a user-facing string.
func HumanReadableHotkey(mods []hotkey.Modifier, key hotkey.Key) string {
	var parts []string
	for _, m := range mods {
		switch m {
		case modCmd:
			parts = append(parts, "Super+")
		case hotkey.ModCtrl:
			parts = append(parts, "Ctrl+")
		case hotkey.ModShift:
			parts = append(parts, "Shift+")
		case modAlt:
			parts = append(parts, "Alt+")
		}
	}
	if name, ok := reverseKeyMap[key]; ok && len(name) > 0 {
		parts = append(parts, strings.ToUpper(name[:1])+name[1:])
	}
	return strings.Join(parts, "")
}
