package stresscase

import (
	"fmt"
	"net"
	"stress/global"
	"stress/mymsg"
	"stress/mysocket"
	"sync"
)

type tryPlayerMgr struct {
	players map[*mysocket.MySocket]int64
	mutex   sync.Mutex
}

var doChan = make(chan uint8)
var tryMgr tryPlayerMgr

func doTryHeart() {

}

func doTryLogin() {
	defer func() {
		doChan <- 0
	}()
	conn, err := net.Dial("tcp", global.AppConfig.ListenIP+":"+global.AppConfig.ListenPort)
	if err != nil {
		fmt.Println(err)
		return
	}
	psocket := mysocket.NewMySocket(conn)
	defer psocket.Close()
	//发送试玩
	tryplay := mymsg.TryPlay{LoginType: 5}
	psocket.Write(&tryplay)
	const readBufferSize = 1024
	var readBuffer = make([]byte, readBufferSize)
	var readedSizes = 0
	for {
		_, err := psocket.Read(readBuffer[readedSizes:])
		if err != nil {
			fmt.Println(err)
			break
		}
	}
}

//StressTryLogin stress try Login
func StressTryLogin(nums int) {
	go doTryHeart()
	for i := 0; i < nums; i++ {
		go doTryLogin()
	}
	for range doChan {
		go doTryLogin()
	}
}
