package stresscase

import (
	"fmt"
	"net"
	"stress/global"
	"stress/head"
	"stress/mymsg"
	"stress/mysocket"
	"sync"
)

type tryPlayerMgr struct {
	players map[*mysocket.MySocket]int64
	mutex   sync.Mutex
}

type userContext struct {
	pdecode *head.Decode
	login   bool
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
		global.AppLog.PrintlnInfo(err)
		return
	}
	psocket := mysocket.NewMySocket(conn)
	defer psocket.Close()
	var pusercontext userContext
	tryplay := mymsg.TryPlay{LoginType: 5}
	psocket.Write(&tryplay)
	const readBufferSize = 10240
	var readBuffer = make([]byte, readBufferSize)
	var readedSizes = 0
	for {
		if readedSizes == readBufferSize {
			global.AppLog.PrintfError("readBuffer reach limit\n")
			break
		}
		n, err := psocket.Read(readBuffer[readedSizes:])
		if err != nil {
			global.AppLog.PrintlnInfo(err)
			break
		}
		readedSizes += n
		procTotal := 0
		for {
			if psocket.IsClose() {
				procTotal = readedSizes
				break
			}
			proc := process(readBuffer[procTotal:readedSizes], psocket, &pusercontext, readBufferSize)
			if proc == 0 {
				break
			}
			procTotal += proc
		}

		if procTotal > 0 {
			copy(readBuffer, readBuffer[procTotal:])
			readedSizes -= procTotal
		}
	}
}

func process(data []byte, psocket *mysocket.MySocket, ucontext *userContext, readBufferSize int) int {
	if len(data) < 4 {
		return 0
	}
	if ucontext.pdecode != nil {
		ucontext.pdecode.Do(data[0:4], data[0:4])
	}
	var h mymsg.Head
	b, s := mymsg.UnSerializeHead(&h, data)
	if !b {
		return 0
	}
	if int(h.Size) < s {
		psocket.Close()
		global.AppLog.PrintfInfo("%#v\n", &h)
		return 0
	}
	if int(h.Size) > readBufferSize {
		psocket.Close()
		global.AppLog.PrintfError("cmdid:%d cmdlen:%d buflen:%d\n", h.Cmdid, h.Size, readBufferSize)
		return 0
	}
	if int(h.Size) > len(data) {
		return 0
	}
	onMsg(h.Cmdid, data[s:h.Size], psocket, ucontext)
	return int(h.Size)
}

func onMsg(cmdid uint16, msg []byte, psocket mysocket.MyWriteCloser, ucontext *userContext) {
	fmt.Println(cmdid)
	if ucontext.login == false {
		if cmdid != mymsg.SMsgTryPlay {
			psocket.Close()
			global.AppLog.PrintfInfo("cmd :%u is not mymsg.SMsgTryPlay \n", cmdid)
			return
		}
		onTryPlayRsp(msg, psocket, ucontext)
		return
	}
	switch cmdid {
	case mymsg.SMsgBaccaratStartBet:
		onBaccaratStartBet(msg, psocket, ucontext)
	}
}

func onTryPlayRsp(msg []byte, psocket mysocket.MyWriteCloser, ucontext *userContext) {
	var rsp mymsg.TryPlayRsp
	b := rsp.UnSerialize(msg)
	if !b {
		psocket.Close()
		return
	}
	fmt.Println(rsp)
	if rsp.Result != 0 {
		psocket.Close()
		return
	}
	ucontext.pdecode = head.NewDecode(rsp.Account + ":" + rsp.Password)
	ucontext.login = true
	psocket.InitSSL(rsp.Account + ":" + rsp.Password)
}

func onBaccaratStartBet(msg []byte, psocket mysocket.MyWriteCloser, ucontext *userContext) {
	var startBet mymsg.BaccaratStartBet
	b := startBet.UnSerialize(msg)
	if !b {
		psocket.Close()
		return
	}
	var sitdown mymsg.SitDown
	sitdown.ServiceID = startBet.ServiceID
	psocket.Write(&sitdown)
	var situp mymsg.SitUp
	situp.ServiceID = startBet.ServiceID
	psocket.Write(&situp)
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
