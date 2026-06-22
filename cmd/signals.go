package cmd

import (
	"os"
	"os/signal"
	"syscall"
)

// watchSignals cancels via stop when SIGINT/SIGTERM is received so in-flight
// HTTP requests are aborted promptly.
func watchSignals(stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		stop()
	}()
}
