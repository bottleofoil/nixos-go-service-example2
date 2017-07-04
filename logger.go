package main

import (
	"fmt"
	"sync"
	"time"
)

type Logger struct {
	mu sync.Mutex
}

func (s *Logger) Info(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts := time.Now().UTC().Format(time.RFC3339)
	fmt.Println(ts, msg)
}

func (s *Logger) Error(msg string) {
	s.Info("ERROR " + msg)
}
