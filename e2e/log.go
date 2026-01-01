package e2e

// Logger is an interface used by e2e test helpers. The testing.T type implements it.
type Logger interface {
	Logf(format string, args ...interface{})
}

// LoggerFunc is an adapter which adapts a printf-style function to the Logger interface. It allows
// using the standard library log.Printf in places where a Logger implementation is expected.
type LoggerFunc func(format string, args ...interface{})

func (f LoggerFunc) Logf(format string, args ...interface{}) {
	f(format, args...)
}
