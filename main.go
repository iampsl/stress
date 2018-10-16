package main

import (
	"log"
	"os"
	"path/filepath"
	"stress/global"
	"stress/stresscase"
)

func init() {
	log.SetFlags(log.Llongfile | log.LstdFlags)
}

func initCwd() {
	err := os.Chdir(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	initCwd()
	if err := global.LoadConfig("config.xml"); err != nil {
		log.Fatalln(err)
	}
	installSignal()
	stresscase.StressTryLogin(5)
}
