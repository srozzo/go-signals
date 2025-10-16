package signals

type LoggerFunc func(format string, args ...any)

type Option func(*Registry)

func WithPolicy(p Policy) Option {
	return func(r *Registry) { r.policy = p }
}

func WithLogger(l LoggerFunc) Option {
	return func(r *Registry) { r.logf = l }
}

func WithDebug(enabled bool) Option {
	return func(r *Registry) { r.debug = enabled }
}
