package main

import (
	"fmt"
	"sync"
)

type logger struct {
	verbose bool
	mu      sync.Mutex
}

func newLogger(verbose bool) *logger {
	return &logger{verbose: verbose}
}

func (l *logger) logf(format string, a ...any) {
	l.mu.Lock()
	fmt.Printf(format, a...)
	l.mu.Unlock()
}

func (l *logger) verboseLogf(format string, a ...any) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	fmt.Printf(format, a...)
	l.mu.Unlock()
}
