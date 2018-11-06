package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"stress/global"
	"stress/stresscase/tryplay"
	"stress/stresscase/userlogin"
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
var cmd = flag.String("cmd", "login", "子命令, 例如try login")

func main() {
	initCwd()
	if err := global.LoadConfig("config.xml"); err != nil {
		log.Fatalln(err)
	}
	installSignal()
	flag.Parse()
	switch *cmd {
	case "try":
		tryplay.StressTryLogin(int(*n))
	case "login":
		userlogin.StressUserLogin()
	default:
		fmt.Printf("未知的命令:%s\n", *cmd)
	}

}
