package main

import (
	"flag"
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

var n = flag.Uint("n", 1, "模拟用户数")

func main() {
	initCwd()
	if err := global.LoadConfig("config.xml"); err != nil {
		log.Fatalln(err)
	}
	installSignal()
	flag.Parse()
	stresscase.StressTryLogin(int(*n))
}
