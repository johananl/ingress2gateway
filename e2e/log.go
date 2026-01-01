package e2e

// Logger is an interface used by e2e test helpers. The testing.T type implements it.
type Logger interface {
	Logf(format string, args ...interface{})
}
