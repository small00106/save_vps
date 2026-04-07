//go:build linux

package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

type ptyTerminalSession struct {
	file      *os.File
	cmd       *exec.Cmd
	closeOnce sync.Once
}

func newTerminalSession(cols, rows int) (terminalSession, error) {
	shell, args, err := interactiveShellCommand()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(shell, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptyFile, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
	if err != nil {
		return nil, err
	}

	return &ptyTerminalSession{
		file: ptyFile,
		cmd:  cmd,
	}, nil
}

func interactiveShellCommand() (string, []string, error) {
	candidates := []struct {
		path string
		args []string
	}{
		{path: "/bin/bash", args: []string{"-i"}},
		{path: "/bin/sh", args: []string{"-i"}},
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate.path); err == nil {
			return candidate.path, candidate.args, nil
		}
	}

	return "", nil, fmt.Errorf("no interactive shell found")
}

func (s *ptyTerminalSession) Read(p []byte) (int, error) {
	return s.file.Read(p)
}

func (s *ptyTerminalSession) Write(p []byte) (int, error) {
	return s.file.Write(p)
}

func (s *ptyTerminalSession) Resize(cols, rows int) error {
	return pty.Setsize(s.file, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
}

func (s *ptyTerminalSession) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		if s.file != nil {
			closeErr = s.file.Close()
		}
		if s.cmd != nil && s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
	})
	return closeErr
}

func (s *ptyTerminalSession) Wait() error {
	if s.cmd == nil {
		return nil
	}
	return s.cmd.Wait()
}
