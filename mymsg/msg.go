package mymsg

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"stress/mybuffer"
)

func myserialize(cmdid uint16, pbuffer *mybuffer.MyBuffer, pmsg interface{}) {
	var h = Head{Cmdid: cmdid}
	begLen := pbuffer.Len()
	pbuffer.AppendUint16(h.Size)
	pbuffer.AppendUint16(h.Cmdid)
	serializeValue(pbuffer, reflect.ValueOf(pmsg).Elem())
	endLen := pbuffer.Len()
	splice := pbuffer.Data()
	binary.LittleEndian.PutUint16(splice[begLen:begLen+2], uint16(endLen-begLen))
}

func myunserialize(data []byte, pmsg interface{}) (bool, int) {
	return unserializeValue(data, reflect.ValueOf(pmsg).Elem())
}

func unserializeValue(data []byte, v reflect.Value) (bool, int) {
	switch v.Kind() {
	case reflect.Bool:
		if len(data) < 1 {
			return false, 0
		}
		var b = uint8(data[0])
		if b == 1 {
			v.SetBool(true)
			return true, 1
		}
		if b == 0 {
			v.SetBool(false)
			return true, 1
		}
		return false, 0
	case reflect.Int8:
		if len(data) < 1 {
			return false, 0
		}
		var b = int64(data[0])
		v.SetInt(b)
		return true, 1
	case reflect.Int16:
		if len(data) < 2 {
			return false, 0
		}
		b16 := binary.LittleEndian.Uint16(data)
		v.SetInt(int64(b16))
		return true, 2
	case reflect.Int32:
		if len(data) < 4 {
			return false, 0
		}
		b32 := binary.LittleEndian.Uint32(data)
		v.SetInt(int64(b32))
		return true, 4
	case reflect.Int64:
		if len(data) < 8 {
			return false, 0
		}
		b64 := binary.LittleEndian.Uint64(data)
		v.SetInt(int64(b64))
		return true, 8
	case reflect.Uint8:
		if len(data) < 1 {
			return false, 0
		}
		b8 := data[0]
		v.SetUint(uint64(b8))
		return true, 1
	case reflect.Uint16:
		if len(data) < 2 {
			return false, 0
		}
		b16 := binary.LittleEndian.Uint16(data)
		v.SetUint(uint64(b16))
		return true, 2
	case reflect.Uint32:
		if len(data) < 4 {
			return false, 0
		}
		b32 := binary.LittleEndian.Uint32(data)
		v.SetUint(uint64(b32))
		return true, 4
	case reflect.Uint64:
		if len(data) < 8 {
			return false, 0
		}
		b64 := binary.LittleEndian.Uint64(data)
		v.SetUint(b64)
		return true, 8
	case reflect.Float32:
		if len(data) < 4 {
			return false, 0
		}
		b32 := binary.LittleEndian.Uint32(data)
		f := math.Float32frombits(b32)
		v.SetFloat(float64(f))
		return true, 4
	case reflect.Float64:
		if len(data) < 8 {
			return false, 0
		}
		b64 := binary.LittleEndian.Uint64(data)
		f := math.Float64frombits(b64)
		v.SetFloat(f)
		return true, 8
	case reflect.Slice:
		if len(data) < 4 {
			return false, 0
		}
		sums := int(binary.LittleEndian.Uint32(data))
		processByte := 4
		v.Set(reflect.MakeSlice(v.Type(), sums, sums))
		for i := 0; i < sums; i++ {
			b, l := unserializeValue(data[processByte:], v.Index(i))
			if !b {
				return false, 0
			}
			processByte += l
		}
		return true, processByte
	case reflect.Map:
		if len(data) < 4 {
			return false, 0
		}
		sums := int(binary.LittleEndian.Uint32(data))
		processByte := 4
		mapType := v.Type()
		keyType := mapType.Key()
		valueType := mapType.Elem()
		v.Set(reflect.MakeMapWithSize(mapType, sums))
		for i := 0; i < sums; i++ {
			newK := reflect.New(keyType)
			b, l := unserializeValue(data[processByte:], newK.Elem())
			if !b {
				return false, 0
			}
			processByte += l
			newV := reflect.New(valueType)
			b, l = unserializeValue(data[processByte:], newV.Elem())
			if !b {
				return false, 0
			}
			processByte += l
			v.SetMapIndex(newK.Elem(), newV.Elem())
		}
		return true, processByte
	case reflect.String:
		i := 0
		for ; i < len(data); i++ {
			if data[i] == 0 {
				break
			}
		}
		if i == len(data) {
			return false, 0
		}
		v.SetString(string(data[0:i]))
		return true, i + 1
	case reflect.Struct:
		processByte := 0
		for i := 0; i < v.NumField(); i++ {
			b, l := unserializeValue(data[processByte:], v.Field(i))
			if !b {
				return false, 0
			}
			processByte += l
		}
		return true, processByte
	default:
		panic(fmt.Sprintf("%v is not support", v.Type()))
	}
}

func serializeValue(pbuffer *mybuffer.MyBuffer, v reflect.Value) {
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			pbuffer.AppendUint8(1)
		} else {
			pbuffer.AppendUint8(0)
		}
	case reflect.Int8:
		pbuffer.AppendInt8(int8(v.Int()))
	case reflect.Int16:
		pbuffer.AppendInt16(int16(v.Int()))
	case reflect.Int32:
		pbuffer.AppendInt32(int32(v.Int()))
	case reflect.Int64:
		pbuffer.AppendInt64(v.Int())
	case reflect.Uint8:
		pbuffer.AppendUint8(uint8(v.Uint()))
	case reflect.Uint16:
		pbuffer.AppendUint16(uint16(v.Uint()))
	case reflect.Uint32:
		pbuffer.AppendUint32(uint32(v.Uint()))
	case reflect.Uint64:
		pbuffer.AppendUint64(v.Uint())
	case reflect.Float32:
		f := math.Float32bits(float32(v.Float()))
		pbuffer.AppendUint32(f)
	case reflect.Float64:
		f := math.Float64bits(v.Float())
		pbuffer.AppendUint64(f)
	case reflect.Slice:
		l := v.Len()
		pbuffer.AppendUint32(uint32(l))
		for i := 0; i < l; i++ {
			serializeValue(pbuffer, v.Index(i))
		}
	case reflect.Map:
		keys := v.MapKeys()
		l := len(keys)
		pbuffer.AppendUint32(uint32(l))
		for i := 0; i < l; i++ {
			serializeValue(pbuffer, keys[i])
			serializeValue(pbuffer, v.MapIndex(keys[i]))
		}
	case reflect.String:
		pbuffer.AppendString(v.String())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			serializeValue(pbuffer, v.Field(i))
		}
	default:
		panic(fmt.Sprintf("%v is not support", v.Type()))
	}
}

//UnSerializeHead 反系列化消息头
func UnSerializeHead(phead *Head, data []byte) (bool, int) {
	if len(data) < 2 {
		return false, 0
	}
	phead.Size = binary.LittleEndian.Uint16(data)
	if len(data[2:]) < 2 {
		return false, 0
	}
	phead.Cmdid = binary.LittleEndian.Uint16(data[2:])
	return true, 4
}

const (
	//CMsgTryPlay 试玩请求
	CMsgTryPlay = 68
	//SMsgTryPlay 试玩响应
	SMsgTryPlay = 69
	//SMsgSignalStatus 信号状态
	SMsgSignalStatus = 58
	//CMsgSitDown 坐下
	CMsgSitDown = 28
	//CMsgAddGolds 下注
	CMsgAddGolds = 110
	//SMsgAddGoldsRsp 下注响应
	SMsgAddGoldsRsp = 111
	//CMsgSitUp 站起
	CMsgSitUp = 31
	//SMsgServerData 游戏数据
	SMsgServerData = 13
	//CMsgSelGroup 选择限额
	CMsgSelGroup = 41
	//SMsgSelGroupRsp 选择限额响应
	SMsgSelGroupRsp = 32
)

//Head 消息头
type Head struct {
	Size  uint16
	Cmdid uint16
}

//TryPlay 试玩
type TryPlay struct {
	LoginType uint8
}

//Serialize 系列化
func (pmsg *TryPlay) Serialize(pbuffer *mybuffer.MyBuffer) {
	myserialize(CMsgTryPlay, pbuffer, pmsg)
}

//UnSerialize 反系列化
func (pmsg *TryPlay) UnSerialize(data []byte) bool {
	b, _ := myunserialize(data, pmsg)
	return b
}

//TryPlayRsp 试玩响应
type TryPlayRsp struct {
	Result   uint8
	Account  string
	Password string
}

//UnSerialize 反系列化
func (pmsg *TryPlayRsp) UnSerialize(data []byte) bool {
	b, _ := myunserialize(data, pmsg)
	return b
}

//SignalStatus 信息状态
type SignalStatus struct {
	ServiceID uint16
	State     uint8
	Time      uint32
	Result    string
}

//UnSerialize 反系列化
func (pmsg *SignalStatus) UnSerialize(data []byte) bool {
	b, _ := myunserialize(data, pmsg)
	return b
}

//SitDown 坐下
type SitDown struct {
	ServiceID uint16
}

//Serialize 系列化
func (pmsg *SitDown) Serialize(pbuffer *mybuffer.MyBuffer) {
	myserialize(CMsgSitDown, pbuffer, pmsg)
}

//BetInfo bet info
type BetInfo struct {
	Type  uint8
	Value uint32
}

//AddGolds 下注
type AddGolds struct {
	ServiceID uint16
	ChairID   uint16
	TableNO   string
	Infos     []BetInfo
}

//Serialize 系列化
func (pmsg *AddGolds) Serialize(pbuffer *mybuffer.MyBuffer) {
	myserialize(CMsgAddGolds, pbuffer, pmsg)
}

//AddGoldsRsp 下注响应
type AddGoldsRsp struct {
	ServiceID uint16
	Code      uint8
	TableNO   string
}

//UnSerialize 反系列化
func (pmsg *AddGoldsRsp) UnSerialize(data []byte) bool {
	b, _ := myunserialize(data, pmsg)
	return b
}

//SitUp 站起
type SitUp struct {
	ServiceID uint16
}

//Serialize 系列化
func (pmsg *SitUp) Serialize(pbuffer *mybuffer.MyBuffer) {
	myserialize(CMsgSitUp, pbuffer, pmsg)
}

//GroupLimit group limit
type GroupLimit struct {
	GroupID  uint16
	MinMoney uint32
	MaxMoney uint32
}

//RoomInfo 房间信息
type RoomInfo struct {
	SortID      uint16
	GameID      uint16
	CatID       uint16
	ServiceID   uint16
	ServerName  string
	TableNO     string
	RoomType    uint8
	Maintain    uint8
	DealerID    string
	DealerName  string
	AnchorID    string //2.6
	AnchorName  string //2.6
	BetTime     uint32
	Tel         string
	Deny        uint8
	VirtableNum uint16 //2.6
	GroupLimits []GroupLimit
}

//UnSerialize 反系列化
func (pmsg *RoomInfo) UnSerialize(data []byte) (bool, int) {
	return myunserialize(data, pmsg)
}

//ZoneLimitInfo 区域限额
type ZoneLimitInfo struct {
	MaxMoney uint32
	MinMoney uint32
	ZoneType uint16
}

//ZoneLimitInfoSlice 切片
type ZoneLimitInfoSlice []ZoneLimitInfo

func (s ZoneLimitInfoSlice) Len() int {
	return len(s)
}

func (s ZoneLimitInfoSlice) Less(i, j int) bool {
	return s[i].ZoneType < s[j].ZoneType
}

func (s ZoneLimitInfoSlice) Swap(i, j int) {
	MaxMoney := s[i].MaxMoney
	MinMoney := s[i].MinMoney
	ZoneType := s[i].ZoneType
	s[i].MaxMoney, s[i].MinMoney, s[i].ZoneType = s[j].MaxMoney, s[j].MinMoney, s[j].ZoneType
	s[j].MaxMoney, s[j].MinMoney, s[j].ZoneType = MaxMoney, MinMoney, ZoneType
}

//BetLimitInfo 赌注限额
type BetLimitInfo struct {
	GroupID   uint16
	GameCatID uint16
	ZoneInfos []ZoneLimitInfo
	RoomType  uint16
}

//UnSerialize 反系列化
func (pmsg *BetLimitInfo) UnSerialize(data []byte) bool {
	b, _ := myunserialize(data, pmsg)
	return b
}

//ServerData 游戏数据
type ServerData struct {
	Data      map[uint16]*RoomInfo
	BetLimits map[uint32]*BetLimitInfo
}

//UnSerialize 反系列化
func (pmsg *ServerData) UnSerialize(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	if pmsg.Data == nil {
		pmsg.Data = make(map[uint16]*RoomInfo)
	}
	count := binary.LittleEndian.Uint16(data)
	if count == 0 {
		return true
	}
	index := 2
	for i := 0; i < int(count); i++ {
		var info RoomInfo
		b, n := info.UnSerialize(data[index:])
		if !b {
			return false
		}
		index += n
		pmsg.Data[info.ServiceID] = &info
	}
	return true
}

//SelGroup 选择限额
type SelGroup struct {
	GroupID   uint16
	GameCatID uint16
	RoomType  uint16
}

//Serialize 系列化
func (pmsg *SelGroup) Serialize(pbuffer *mybuffer.MyBuffer) {
	myserialize(CMsgSelGroup, pbuffer, pmsg)
}
