package tui

import (
	"github.com/pkg/browser"
)

// set up browser logging
func init() {
	browser.Stdout = loggingWriter("browser-stdout")
	browser.Stderr = loggingWriter("browser-stderr")
}
