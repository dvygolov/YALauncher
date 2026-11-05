package index

// debug is set by the lnkdbg tool; normal builds leave it nil so the parser
// stays free of logging.
var debug func(format string, args ...any)

func debugf(format string, args ...any) {
	if debug != nil {
		debug(format, args...)
	}
}

// EnableDebug turns the parser trace on; pass nil to print to stdout.
func EnableDebug(f func(format string, args ...any)) {
	if f == nil {
		f = func(format string, args ...any) { println(sprintf(format, args...)) }
	}
	debug = f
}
