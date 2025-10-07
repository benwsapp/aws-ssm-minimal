// Package supervisor manages the lifecycle of the wrapped service process.
package supervisor

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

const (
	defaultSignalBuffer = 4
	defaultKillOffset   = 128
)

// Result captures the outcome of supervising the child process.
type Result struct {
	ExitCode   int
	TTLExpired bool
}

// Supervisor coordinates TTL enforcement and signal forwarding for a process.
type Supervisor struct {
	cmd           *exec.Cmd
	ttlTimer      *time.Timer
	ttlDuration   time.Duration
	shutdownGrace time.Duration
	done          chan error
	sigs          chan os.Signal
	graceTimer    *time.Timer
	ttlExpired    bool
}

// Run starts the given command and enforces TTL and graceful shutdown behavior.
func Run(cmd *exec.Cmd, ttl, shutdownGrace time.Duration) (Result, error) {
	s := NewSupervisor(cmd, ttl, shutdownGrace)

	return s.Run()
}

// NewSupervisor constructs a Supervisor instance.
func NewSupervisor(cmd *exec.Cmd, ttl, shutdownGrace time.Duration) *Supervisor {
	return &Supervisor{
		cmd:           cmd,
		ttlTimer:      time.NewTimer(ttl),
		ttlDuration:   ttl,
		shutdownGrace: shutdownGrace,
		done:          make(chan error, 1),
		sigs:          make(chan os.Signal, defaultSignalBuffer),
		graceTimer:    nil,
		ttlExpired:    false,
	}
}

// Run supervises the configured process until it exits or the TTL elapses.
func (s *Supervisor) Run() (Result, error) {
	startErr := s.start()
	if startErr != nil {
		return Result{}, startErr
	}

	defer s.cleanup()

	return s.eventLoop()
}

func (s *Supervisor) start() error {
	startErr := s.cmd.Start()
	if startErr != nil {
		return fmt.Errorf("start child process: %w", startErr)
	}

	log.Printf("started child process pid=%d ttl=%s", s.cmd.Process.Pid, s.ttlDuration)

	go func() {
		s.done <- s.cmd.Wait()
	}()

	signal.Notify(s.sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)

	return nil
}

func (s *Supervisor) eventLoop() (Result, error) {
	for {
		select {
		case err := <-s.done:
			return s.handleProcessExit(err)
		case sig := <-s.sigs:
			s.forwardSignal(sig)
		case <-s.ttlTimer.C:
			s.handleTTLExpiry()
		}
	}
}

func (s *Supervisor) handleProcessExit(err error) (Result, error) {
	s.stopTimer(s.graceTimer)

	if s.ttlExpired {
		log.Printf("child exited after ttl expiry")

		return Result{ExitCode: 0, TTLExpired: true}, nil
	}

	exitCode, exitErr := exitCodeFromError(err)

	return Result{ExitCode: exitCode, TTLExpired: false}, exitErr
}

func (s *Supervisor) forwardSignal(sig os.Signal) {
	log.Printf("forwarding signal %s to child", sig)

	if s.cmd.Process == nil {
		return
	}

	signalErr := s.cmd.Process.Signal(sig)
	if signalErr != nil {
		log.Printf("warning: failed to forward signal %s: %v", sig, signalErr)
	}
}

func (s *Supervisor) handleTTLExpiry() {
	s.ttlExpired = true
	log.Printf("ttl expired; sending SIGTERM and waiting %s before SIGKILL", s.shutdownGrace)
	s.signalChild(syscall.SIGTERM)
	s.scheduleKill()
}

func (s *Supervisor) scheduleKill() {
	if s.shutdownGrace <= 0 {
		log.Printf("grace period is zero; sending SIGKILL immediately")
		s.signalChild(syscall.SIGKILL)

		return
	}

	s.graceTimer = time.AfterFunc(s.shutdownGrace, func() {
		log.Printf("grace period elapsed; sending SIGKILL to child")
		s.signalChild(syscall.SIGKILL)
	})
}

func (s *Supervisor) signalChild(sig os.Signal) {
	if s.cmd.Process == nil {
		return
	}

	signalErr := s.cmd.Process.Signal(sig)
	if signalErr != nil {
		log.Printf("warning: failed to send %s to child: %v", sig, signalErr)
	}
}

func (s *Supervisor) cleanup() {
	s.stopTimer(s.ttlTimer)
	s.stopTimer(s.graceTimer)
	signal.Stop(s.sigs)
}

func (s *Supervisor) stopTimer(timer *time.Timer) {
	if timer == nil {
		return
	}

	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func exitCodeFromError(err error) (int, error) {
	if err == nil {
		return 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() {
				sig := status.Signal()
				log.Printf("child terminated by signal %s", sig)

				return defaultKillOffset + int(sig), nil
			}

			return status.ExitStatus(), nil
		}

		return exitErr.ExitCode(), nil
	}

	return 1, fmt.Errorf("child process error: %w", err)
}
