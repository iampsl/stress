package main

import (
	"os"
	"os/signal"
	"stress/global"
	"syscall"
)

var sigs = make(chan os.Signal, 1)

func installSignal() {
	signal.Notify(sigs, syscall.SIGUSR1)
	go procSignal()
}

func procSignal() {
	for {
		<-sigs
		global.AppLog.SwitchInfo()
	}
}
