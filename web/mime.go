package web

import "mime"

func init() {
	// Windows may serve .mjs as text/plain from the registry.
	// Browsers reject dynamic module imports unless the MIME type is a
	// JavaScript type, so force the correct mapping.
	mime.AddExtensionType(".mjs", "application/javascript")
}
