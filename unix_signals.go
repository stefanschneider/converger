// +build !windows

package main

import (
	"os"
	"syscall"
)

func init() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGUSR2)

	go func() {
		for sig := range c {
			if sig == os.Interrupt || sig == os.Kill {
				stopSignal <- true
			}
			if sig == syscall.SIGUSR2 {
				drainSignal <- true
			}
		}
	}()
}
