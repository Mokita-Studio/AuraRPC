// Package icons embeds the tray icon PNGs for both Windows taskbar
// themes. ForDark* targets dark taskbars (light-toned glyph),
// ForLight* targets light taskbars (dark-toned glyph).
package icons

import _ "embed"

//go:embed dark_16.png
var ForDark16 []byte

//go:embed dark_24.png
var ForDark24 []byte

//go:embed light_16.png
var ForLight16 []byte

//go:embed light_24.png
var ForLight24 []byte
