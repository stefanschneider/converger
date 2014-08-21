// +build windows

package main

import (
	"code.google.com/p/winsvc/svc"
	"code.google.com/p/winsvc/winapi"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

type WindowsService struct {
	started chan bool
}

var stdoutFile *os.File
var stderrFile *os.File

func (ws *WindowsService) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	ws.started <- true

	s <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

loop:
	for {
		select {
		case change := <-r:
			switch change.Cmd {
			case svc.Interrogate:
				s <- change.CurrentStatus
			case svc.Stop, svc.Shutdown:
				{
					stopSignal <- true

					break loop
				}
			default:
				{
					break loop
				}
			}
		}
	}

	s <- svc.Status{State: svc.StopPending}

	return
}

func init() {
	drainEventName := "cf-converger-drain"

	redirectStdStreams()

	if interactive, _ := svc.IsAnInteractiveSession(); interactive {
		log.Print("Interactive mode")

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		go func() {
			for sig := range c {
				if sig == os.Interrupt || sig == os.Kill {
					stopSignal <- true
				}
			}
		}()

	} else {
		log.Print("Service mode")

		// Use global named object so that the Event could be signaled from an interactive seession
		drainEventName = "Global\\" + drainEventName

		waitForStart := make(chan bool)
		ws := WindowsService{started: waitForStart}

		go svc.Run("cf-converger", &ws)

		log.Print("Waiting for start")

		<-waitForStart

		log.Print("Done waiting for start")
	}

	// To singal the event use this PS script:
	// $drain_event = New-Object -TypeName System.Threading.EventWaitHandle -ArgumentList false, ([System.Threading.EventResetMode]::AutoReset) , "cf-converger-drain"
	// $drain_event = New-Object -TypeName System.Threading.EventWaitHandle -ArgumentList false, ([System.Threading.EventResetMode]::AutoReset) , "Global\cf-converger-drain"
	// $drain_event.Set()

	drianEvent, err := newNamedEvent(drainEventName)
	if err != nil {
		log.Fatal("Error creating Event", err)
	}

	go func() {
		for {
			err := drianEvent.Wait()
			if err != nil {
				log.Fatal("Error waiting for Event", err)
			}
			drainSignal <- true
		}

	}()
}

func redirectStdStreams() {
	stdoutFilePath := ""
	stderrFilePath := ""

	flag.StringVar(
		&stdoutFilePath,
		"stdoutFile",
		"",
		"Path of file where to redirect stdout stream",
	)

	flag.StringVar(
		&stderrFilePath,
		"stderrFile",
		"",
		"Path of file where to redirect stderr stream",
	)

	if !flag.Parsed() {
		flag.Parse()
	}

	var err error = nil

	if stdoutFilePath != "" {
		stdoutFile, err = os.OpenFile(stdoutFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND|os.O_SYNC, 0660)
		if err != nil {
			log.Fatal("Error opening stdoutFile", err)
		}
		os.Stdout = stdoutFile
	}
	if stderrFilePath != "" {
		if stderrFilePath != stdoutFilePath {
			stderrFile, err = os.OpenFile(stderrFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND|os.O_SYNC, 0660)
			if err != nil {
				log.Fatal("Error opening stderrFile", err)
			}
		} else {
			stderrFile = stdoutFile
		}
		os.Stderr = stderrFile
	}
}

// Code adapted from here: https://code.google.com/p/winsvc/source/browse/svc/event.go
// Put this into another package ?
type namedEvent struct {
	h syscall.Handle
}

func newNamedEvent(name string) (*namedEvent, error) {
	pname, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}

	h, err := winapi.CreateEvent(nil, 0, 0, pname)
	if err != nil {
		return nil, err
	}
	return &namedEvent{h: h}, nil
}

func (e *namedEvent) Close() error {
	return syscall.CloseHandle(e.h)
}

func (e *namedEvent) Set() error {
	return winapi.SetEvent(e.h)
}

func (e *namedEvent) Wait() error {
	s, err := syscall.WaitForSingleObject(e.h, syscall.INFINITE)
	switch s {
	case syscall.WAIT_OBJECT_0:
		break
	case syscall.WAIT_FAILED:
		return err
	default:
		return errors.New("unexpected result from WaitForSingleObject")
	}
	return nil
}
