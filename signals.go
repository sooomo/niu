package niu

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// wait system signals.
//
// when signal comes, do some cleanups
func WaitSysSignal(cleanup func()) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	fmt.Printf("Receive system signal: %v\n", sig)

	cleanup()

	os.Exit(0)
}
