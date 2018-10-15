package mysocket

import (
	"net"
	"stress/head"
	"stress/mybuffer"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

//MyWriteCloser 接口
type MyWriteCloser interface {
	Close()
	WriteBytes(b []byte)
	Write(s Serializer)
	InitSSL(key string)
}

//MySocket 对net.Conn 的包装
type MySocket struct {
	conn      net.Conn
	buffers   [2]*mybuffer.MyBuffer
	sendIndex uint
	notify    chan int
	isclose   uint32

	m          sync.Mutex
	bclose     bool
	writeIndex uint
	pencode    *head.Encode
}

//NewMySocket 创建一个MySocket
func NewMySocket(c net.Conn) *MySocket {
	if c == nil {
		panic("c is nil")
	}
	var psocket = new(MySocket)
	psocket.conn = c
	psocket.buffers[0] = new(mybuffer.MyBuffer)
	psocket.buffers[1] = new(mybuffer.MyBuffer)
	psocket.sendIndex = 0
	psocket.notify = make(chan int, 1)
	psocket.isclose = 0
	psocket.bclose = false
	psocket.writeIndex = 1
	go doMySocketSend(psocket)
	return psocket
}

func doMySocketSend(my *MySocket) {
	writeErr := false
	for {
		_, ok := <-my.notify
		if !ok {
			return
		}
		time.Sleep(100 * time.Millisecond)
		my.m.Lock()
		my.writeIndex = my.sendIndex
		my.m.Unlock()
		my.sendIndex = (my.sendIndex + 1) % 2
		if !writeErr {
			var sendSplice = my.buffers[my.sendIndex].Data()
			for len(sendSplice) > 0 {
				n, err := my.conn.Write(sendSplice)
				if err != nil {
					writeErr = true
					break
				}
				sendSplice = sendSplice[n:]
			}
		}
		my.buffers[my.sendIndex].Clear()
	}
}

//Read 读数据
func (my *MySocket) Read(b []byte) (n int, err error) {
	return my.conn.Read(b)
}

//WriteBytes 写数据
func (my *MySocket) WriteBytes(b []byte) {
	if len(b) == 0 {
		return
	}
	my.m.Lock()
	if my.bclose {
		my.m.Unlock()
		return
	}
	var dataLen = my.buffers[my.writeIndex].Len()
	my.buffers[my.writeIndex].Append(b...)
	if dataLen == 0 {
		my.notify <- 0
	}
	my.m.Unlock()
}

//Serializer 系列化接口
type Serializer interface {
	Serialize(pbuffer *mybuffer.MyBuffer)
}

//Write 写数据
func (my *MySocket) Write(s Serializer) {
	if s == nil {
		return
	}
	my.m.Lock()
	if my.bclose {
		my.m.Unlock()
		return
	}
	var dataLen = my.buffers[my.writeIndex].Len()
	s.Serialize(my.buffers[my.writeIndex])
	var nowDataLen = my.buffers[my.writeIndex].Len()
	if nowDataLen-dataLen >= 4 {
		if my.pencode != nil {
			tmpData := my.buffers[my.writeIndex].Data()
			my.pencode.Do(tmpData[dataLen:dataLen+4], tmpData[dataLen:dataLen+4])
		}
	}
	if dataLen == 0 && nowDataLen != 0 {
		my.notify <- 0
	}
	my.m.Unlock()
}

//InitSSL 初始化ssl
func (my *MySocket) InitSSL(key string) {
	my.m.Lock()
	my.pencode = head.NewEncode(key)
	my.m.Unlock()
}

//Close 关闭一个MySocket, 释放系统资源
func (my *MySocket) Close() {
	my.m.Lock()
	if my.bclose {
		my.m.Unlock()
		return
	}
	my.bclose = true
	my.conn.Close()
	close(my.notify)
	my.m.Unlock()
	atomic.StoreUint32(&(my.isclose), 1)
}

//IsClose 判断MySocket是否关闭
func (my *MySocket) IsClose() bool {
	val := atomic.LoadUint32(&(my.isclose))
	if val > 0 {
		return true
	}
	return false
}

//MyWebSocket 对websocket.Conn 的包装
type MyWebSocket struct {
	conn      *websocket.Conn
	buffers   [2]*mybuffer.MyBuffer
	sinfo     [2][]uint32
	sendIndex uint
	notify    chan int

	m          sync.Mutex
	bclose     bool
	writeIndex uint
}

//NewMyWebSocket 创建一个MyWebSocket
func NewMyWebSocket(c *websocket.Conn) *MyWebSocket {
	if c == nil {
		panic("c is nil")
	}
	var psocket = new(MyWebSocket)
	psocket.conn = c
	psocket.buffers[0] = new(mybuffer.MyBuffer)
	psocket.buffers[1] = new(mybuffer.MyBuffer)
	psocket.sendIndex = 0
	psocket.notify = make(chan int, 1)
	psocket.bclose = false
	psocket.writeIndex = 1
	go doMyWebSocketSend(psocket)
	return psocket
}

func doMyWebSocketSend(my *MyWebSocket) {
	writeErr := false
	for {
		_, ok := <-my.notify
		if !ok {
			return
		}
		my.m.Lock()
		my.writeIndex = my.sendIndex
		my.m.Unlock()
		my.sendIndex = (my.sendIndex + 1) % 2
		if !writeErr {
			var sendSplice = my.buffers[my.sendIndex].Data()
			var sizeSplice = my.sinfo[my.sendIndex]
			for _, v := range sizeSplice {
				err := my.conn.WriteMessage(websocket.BinaryMessage, sendSplice[0:v])
				if err != nil {
					writeErr = true
					break
				}
				sendSplice = sendSplice[v:]
			}
		}
		my.buffers[my.sendIndex].Clear()
		my.sinfo[my.sendIndex] = my.sinfo[my.sendIndex][0:0]
	}
}

//ReadMessage 读数据
func (my *MyWebSocket) ReadMessage() (msgtype int, data []byte, err error) {
	return my.conn.ReadMessage()
}

//WriteBytes 写数据
func (my *MyWebSocket) WriteBytes(b []byte) {
	if len(b) == 0 {
		return
	}
	my.m.Lock()
	if my.bclose {
		my.m.Unlock()
		return
	}
	var dataLen = my.buffers[my.writeIndex].Len()
	my.buffers[my.writeIndex].Append(b...)
	my.sinfo[my.writeIndex] = append(my.sinfo[my.writeIndex], uint32(len(b)))
	if dataLen == 0 {
		my.notify <- 0
	}
	my.m.Unlock()
}

//Write 写数据
func (my *MyWebSocket) Write(s Serializer) {
	if s == nil {
		return
	}
	my.m.Lock()
	if my.bclose {
		my.m.Unlock()
		return
	}
	var dataLen = my.buffers[my.writeIndex].Len()
	s.Serialize(my.buffers[my.writeIndex])
	var nowDataLen = my.buffers[my.writeIndex].Len()
	sub := nowDataLen - dataLen
	if sub != 0 {
		my.sinfo[my.writeIndex] = append(my.sinfo[my.writeIndex], uint32(sub))
	}
	if dataLen == 0 && nowDataLen != 0 {
		my.notify <- 0
	}
	my.m.Unlock()
}

//Close 关闭一个MySocket, 释放系统资源
func (my *MyWebSocket) Close() {
	my.m.Lock()
	if my.bclose {
		my.m.Unlock()
		return
	}
	my.bclose = true
	my.conn.Close()
	close(my.notify)
	my.m.Unlock()
}
