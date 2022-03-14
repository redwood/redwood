package embed

import (
	_ "embed"
)

//go:embed node_modules/@redwood.dev/client/browser/index.js
var BrowserSrc []byte
