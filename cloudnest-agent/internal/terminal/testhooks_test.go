package terminal

func swapNewTerminalSessionFunc(fn func(cols, rows int) (terminalSession, error)) func() {
	prev := newTerminalSessionFunc
	newTerminalSessionFunc = fn
	return func() {
		newTerminalSessionFunc = prev
	}
}
