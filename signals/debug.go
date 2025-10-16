package signals

// Debug helpers kept for compatibility with earlier revisions.
// The canonical debug switch now lives on the Default registry
// and is exposed via SetDebug in convenience.go.

// isDebug reports whether debug logging is currently enabled
// on the Default registry. It is for internal use only.
func isDebug() bool {
	return Default.debug
}

// setDebug is an internal helper retained for transitional use.
// Prefer the exported SetDebug in convenience.go.
func setDebug(enabled bool) {
	SetDebug(enabled)
}
