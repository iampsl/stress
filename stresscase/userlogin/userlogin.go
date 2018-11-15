package userlogin

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"stress/global"
	"stress/head"
	"stress/mymsg"
	"stress/mysocket"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type userMgr struct {
	users map[*mysocket.MySocket]*int64
	m     sync.Mutex
}

func (mgr *userMgr) add(psocket *mysocket.MySocket, plastTime *int64) {
	mgr.m.Lock()
	if mgr.users == nil {
		mgr.users = make(map[*mysocket.MySocket]*int64)
	}
	mgr.users[psocket] = plastTime
	mgr.m.Unlock()
}

func (mgr *userMgr) del(psocket *mysocket.MySocket) {
	mgr.m.Lock()
	delete(mgr.users, psocket)
	mgr.m.Unlock()
}

func (mgr *userMgr) close(timeout int) {
	now := time.Now().Unix()
	mgr.m.Lock()
	for k, v := range mgr.users {
		if now > atomic.LoadInt64(v)+int64(timeout) {
			k.Close()
		}
	}
	mgr.m.Unlock()
}

var umgr userMgr

func doTimeoutCheck() {
	for {
		time.Sleep(30 * time.Second)
		umgr.close(60)
	}
}

type userContext struct {
	pdecode      *head.Decode
	login        bool
	decode       bool
	gamedata     mymsg.ServerData
	keys         userPasswd
	lastReadTime int64
}

type userPasswd struct {
	username  string
	passwd    string
	agentcode string
}

var doChan = make(chan userPasswd)

func userLogin(info userPasswd) {
	defer func() {
		doChan <- info
	}()
	conn, err := net.Dial("tcp", global.AppConfig.ListenIP+":"+global.AppConfig.ListenPort)
	if err != nil {
		global.AppLog.PrintlnInfo(err)
		return
	}
	psocket := mysocket.NewMySocket(conn)
	defer func() {
		psocket.Close()
		umgr.del(psocket)
	}()
	var pusercontext userContext
	pusercontext.gamedata.BetLimits = make(map[uint32]*mymsg.BetLimitInfo)
	pusercontext.keys = info
	pusercontext.lastReadTime = time.Now().Unix()
	umgr.add(psocket, &pusercontext.lastReadTime)
	login := mymsg.AuthSession{Version: 12340, LoginType: 3, IP: "192.168.0.0", Account: info.username, Passwd: info.passwd, AgentCode: info.agentcode}
	psocket.Write(&login)
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
	if ucontext.pdecode != nil && ucontext.decode == false {
		ucontext.pdecode.Do(data[0:4], data[0:4])
		ucontext.decode = true
	}
	var h mymsg.Head
	b, s := mymsg.UnSerializeHead(&h, data[:])
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
	ucontext.decode = false
	onMsg(h.Cmdid, data[s:h.Size], psocket, ucontext)
	return int(h.Size)
}

func onMsg(cmdid uint16, msg []byte, psocket mysocket.MyWriteCloser, ucontext *userContext) {
	now := time.Now().Unix()
	atomic.StoreInt64(&(ucontext.lastReadTime), now)
	if ucontext.login == false {
		if cmdid != mymsg.SMsgAuthSessionRsp {
			psocket.Close()
			global.AppLog.PrintfInfo("cmd :%u is not mymsg.SMsgAuthSessionRsp \n", cmdid)
			return
		}
		onAuthSessionRsp(msg, psocket, ucontext)
		return
	}
	switch cmdid {
	case mymsg.SMsgSignalStatus:
		onSignalStatus(msg, psocket, ucontext)
	case mymsg.SMsgServerData:
		onServerData(msg, psocket, ucontext)
	case mymsg.SMsgSelGroupRsp:
		onSelGroupRsp(msg, psocket, ucontext)
	case mymsg.SMsgAddGoldsRsp:
		onAddGoldRsp(msg, psocket, ucontext)
	}
}

func onAuthSessionRsp(msg []byte, psocket mysocket.MyWriteCloser, ucontext *userContext) {
	var rsp mymsg.AuthSessionRsp
	b := rsp.UnSerialize(msg)
	if !b {
		psocket.Close()
		return
	}
	if rsp.Result != 12 {
		psocket.Close()
		return
	}
	ucontext.pdecode = head.NewDecode(ucontext.keys.username + ":" + ucontext.keys.passwd)
	ucontext.login = true
	psocket.InitSSL(ucontext.keys.username + ":" + ucontext.keys.passwd)
}

func onSignalStatus(msg []byte, psocket mysocket.MyWriteCloser, ucontext *userContext) {
	var sigStatus mymsg.SignalStatus
	b := sigStatus.UnSerialize(msg)
	if !b {
		psocket.Close()
		return
	}
	if sigStatus.State != 1 {
		return
	}
	var sitdown mymsg.SitDown
	sitdown.ServiceID = sigStatus.ServiceID
	psocket.Write(&sitdown)
	info, ok := ucontext.gamedata.Data[sigStatus.ServiceID]
	if ok {
		limitInfos := ucontext.gamedata.BetLimits[(uint32(info.RoomType)<<16)|uint32(info.CatID)]
		if limitInfos != nil && len(limitInfos.ZoneInfos) > 0 {
			var addgolds mymsg.AddGolds
			addgolds.ServiceID = sigStatus.ServiceID
			addgolds.ChairID = 1
			addgolds.TableNO = info.TableNO
			addgolds.Infos = []mymsg.BetInfo{mymsg.BetInfo{Type: uint8(limitInfos.ZoneInfos[0].ZoneType), Value: limitInfos.ZoneInfos[0].MinMoney}}
			psocket.Write(&addgolds)
		} else if len(info.GroupLimits) > 0 {
			var selGroup mymsg.SelGroup
			selGroup.GroupID = info.GroupLimits[0].GroupID
			selGroup.RoomType = uint16(info.RoomType)
			selGroup.GameCatID = info.CatID
			psocket.Write(&selGroup)
		}
	}
	var situp mymsg.SitUp
	situp.ServiceID = sigStatus.ServiceID
	psocket.Write(&situp)
}

func onServerData(msg []byte, psocket mysocket.MyWriteCloser, ucontext *userContext) {
	if len(msg) < 4 {
		return
	}
	b := bytes.NewReader(msg[4:])
	r, err := zlib.NewReader(b)
	if err != nil {
		global.AppLog.PrintlnInfo(err)
		return
	}
	var out bytes.Buffer
	_, err = io.Copy(&out, r)
	if err != nil {
		global.AppLog.PrintlnInfo(err)
		return
	}
	ucontext.gamedata.UnSerialize(out.Bytes())
	for _, v := range ucontext.gamedata.Data {
		splits := strings.Split(v.TableNO, ";")
		if len(splits) > 0 {
			v.TableNO = splits[0]
		}
	}
}

func onSelGroupRsp(msg []byte, psocket mysocket.MyWriteCloser, ucontext *userContext) {
	var rsp mymsg.BetLimitInfo
	b := rsp.UnSerialize(msg)
	if !b {
		global.AppLog.PrintfError("UnSerialize failed\n")
		return
	}
	key := (uint32(rsp.RoomType) << 16) | uint32(rsp.GameCatID)
	if pvalue := ucontext.gamedata.BetLimits[key]; pvalue != nil {
		return
	}
	sort.Sort(mymsg.ZoneLimitInfoSlice(rsp.ZoneInfos))
	ucontext.gamedata.BetLimits[key] = &rsp
}

func onAddGoldRsp(msg []byte, psocket mysocket.MyWriteCloser, ucontext *userContext) {
	var rsp mymsg.AddGoldsRsp
	b := rsp.UnSerialize(msg)
	if !b {
		return
	}
	if rsp.Code == 37 {
		psocket.Close()
	}
}

//StressUserLogin stress user Login
func StressUserLogin() {
	file, err := os.Open("user.txt")
	if err != nil {
		fmt.Println(err)
		return
	}
	users := make([]userPasswd, 0, 1000)
	input := bufio.NewScanner(file)
	for input.Scan() {
		tmpStrings := strings.Fields(input.Text())
		if len(tmpStrings) < 3 {
			continue
		}
		if len(tmpStrings[0]) == 0 || len(tmpStrings[1]) == 0 || len(tmpStrings[2]) == 0 {
			continue
		}
		users = append(users, userPasswd{tmpStrings[0], tmpStrings[1], tmpStrings[2]})
	}
	err = input.Err()
	if err != nil {
		fmt.Println(err)
	}
	file.Close()
	if len(users) <= 0 {
		fmt.Printf("没有得到用户信息\n")
		return
	}
	go doTimeoutCheck()
	for i, v := range users {
		go userLogin(v)
		if 0 == i%200 {
			time.Sleep(time.Second)
		}
	}
	for info := range doChan {
		go userLogin(info)
	}
}
