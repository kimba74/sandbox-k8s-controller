package main

import (
	"os"
	"os/signal"
	"syscall"
)

func setupSignls() <-chan struct{} {
	stop := make(chan struct{})
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		close(stop)
		<-ch
		os.Exit(1)
	}()
	return stop
}
