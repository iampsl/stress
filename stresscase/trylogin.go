package stresscase

import (
	"bytes"
	"compress/zlib"
	"io"
	"net"
	"stress/global"
	"stress/head"
	"stress/mymsg"
	"stress/mysocket"
	"strings"
	"sync"
)

type tryPlayerMgr struct {
	players map[*mysocket.MySocket]int64
	mutex   sync.Mutex
}

type selGroupInfo struct {
	groupID   uint16
	gameCatID uint16
	roomType  uint16
}

type userContext struct {
	pdecode       *head.Decode
	login         bool
	decode        bool
	gamedata      mymsg.ServerData
	selGroupInfos []selGroupInfo
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

func onTryPlayRsp(msg []byte, psocket mysocket.MyWriteCloser, ucontext *userContext) {
	var rsp mymsg.TryPlayRsp
	b := rsp.UnSerialize(msg)
	if !b {
		psocket.Close()
		return
	}
	if rsp.Result != 0 {
		psocket.Close()
		return
	}
	ucontext.pdecode = head.NewDecode(rsp.Account + ":" + rsp.Password)
	ucontext.login = true
	psocket.InitSSL(rsp.Account + ":" + rsp.Password)
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
		if ucontext.gamedata.BetLimits != nil {
			limitInfos := ucontext.gamedata.BetLimits[(uint32(info.RoomType)<<16)|uint32(info.CatID)]
			if limitInfos != nil && len(limitInfos.ZoneInfos) > 0 {
				var addgolds mymsg.AddGolds
				addgolds.ServiceID = sigStatus.ServiceID
				addgolds.ChairID = 1
				addgolds.TableNO = info.TableNO
				addgolds.Infos = []mymsg.BetInfo{mymsg.BetInfo{Type: uint8(limitInfos.ZoneInfos[0].ZoneType), Value: limitInfos.ZoneInfos[0].MinMoney}}
				psocket.Write(&addgolds)
			}
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
		roomType := v.RoomType
		gameCatID := v.CatID
		groupID := uint16(0)
		if len(v.GroupLimits) <= 0 {
			continue
		}
		groupID = v.GroupLimits[0].GroupID
		bfind := false
		for _, tmp := range ucontext.selGroupInfos {
			if tmp.gameCatID == gameCatID && tmp.groupID == groupID && tmp.roomType == uint16(roomType) {
				bfind = true
				break
			}
		}
		if !bfind {
			ucontext.selGroupInfos = append(ucontext.selGroupInfos, selGroupInfo{groupID, gameCatID, uint16(roomType)})
		}
	}
	if len(ucontext.selGroupInfos) > 0 {
		var selGroup mymsg.SelGroup
		selGroup.GroupID = ucontext.selGroupInfos[0].groupID
		selGroup.RoomType = ucontext.selGroupInfos[0].roomType
		selGroup.GameCatID = ucontext.selGroupInfos[0].gameCatID
		psocket.Write(&selGroup)
	}
}

func onSelGroupRsp(msg []byte, psocket mysocket.MyWriteCloser, ucontext *userContext) {
	var rsp mymsg.BetLimitInfo
	b := rsp.UnSerialize(msg)
	if !b {
		global.AppLog.PrintfError("UnSerialize failed\n")
		return
	}
	if len(ucontext.selGroupInfos) <= 0 {
		return
	}
	roomType := ucontext.selGroupInfos[0].roomType
	gameCatID := ucontext.selGroupInfos[0].gameCatID
	if ucontext.gamedata.BetLimits == nil {
		ucontext.gamedata.BetLimits = make(map[uint32]*mymsg.BetLimitInfo)
	}
	ucontext.gamedata.BetLimits[(uint32(roomType)<<16)|uint32(gameCatID)] = &rsp

	ucontext.selGroupInfos = ucontext.selGroupInfos[1:]
	if len(ucontext.selGroupInfos) > 0 {
		var selGroup mymsg.SelGroup
		selGroup.GroupID = ucontext.selGroupInfos[0].groupID
		selGroup.RoomType = ucontext.selGroupInfos[0].roomType
		selGroup.GameCatID = ucontext.selGroupInfos[0].gameCatID
		psocket.Write(&selGroup)
	}
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
