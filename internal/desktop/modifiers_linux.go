//go:build linux

package desktop

import "golang.design/x/hotkey"

// modCmd maps to the Super/Meta key (Mod4) on Linux.
var modCmd = hotkey.Mod4

// modAlt maps to the Alt key (Mod1) on Linux.
var modAlt = hotkey.Mod1
