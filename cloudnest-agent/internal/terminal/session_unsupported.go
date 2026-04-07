//go:build !linux

package terminal

func newTerminalSession(cols, rows int) (terminalSession, error) {
	return nil, ErrTerminalUnsupported
}
