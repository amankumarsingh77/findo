//go:build windows

package desktop

import "golang.design/x/hotkey"

// modCmd maps to the Windows key (ModWin) on Windows.
var modCmd = hotkey.ModWin

// modAlt maps to the Alt key on Windows.
var modAlt = hotkey.ModAlt
