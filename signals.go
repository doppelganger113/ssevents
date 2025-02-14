package sse_server

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// WatchSigTerm - sends an error on termination signal, eg ctrl+c, on second signal panics
func WatchSigTerm() <-chan error {
	err := make(chan error)
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		err <- fmt.Errorf("%s", <-c)
		<-c
		panic("ctrl+c called twice, force exiting")
	}()

	return err
}
