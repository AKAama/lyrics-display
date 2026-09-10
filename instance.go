package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

var errAlreadyRunning = errors.New("another lyrics-display instance is already running")

var instanceLockFile *os.File

func acquireInstanceLock(store *configStore) error {
	if err := store.ensureDir(); err != nil {
		return err
	}

	path := filepath.Join(filepath.Dir(store.pathString()), "instance.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return errAlreadyRunning
	}

	if err := f.Truncate(0); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return err
	}
	if _, err := fmt.Fprintf(f, "%d\n", os.Getpid()); err != nil {
		f.Close()
		return err
	}

	instanceLockFile = f
	return nil
}

func newAppLogger() *log.Logger {
	prefix := "[lyrics-display] "
	flags := log.LstdFlags

	logPath, err := appLogPath()
	if err != nil {
		return log.New(os.Stdout, prefix, flags)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return log.New(os.Stdout, prefix, flags)
	}

	if isTerminal(os.Stdout) {
		return log.New(io.MultiWriter(os.Stdout, f), prefix, flags)
	}
	return log.New(f, prefix, flags)
}

func appLogPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	logDir := filepath.Join(dir, appName)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(logDir, "lyrics-display.log"), nil
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func notifyAlreadyRunning() {
	_ = exec.Command("osascript", "-e", `display notification "lyrics-display 已经在运行" with title "lyrics-display"`).Run()
}
