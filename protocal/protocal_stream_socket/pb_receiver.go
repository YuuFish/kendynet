package protocal_stream_socket

import (
	"github.com/sniperHW/kendynet"
//	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet/util/pb"
//	"fmt"
//	"encoding/binary"
)

type PBReceiver struct {
	recvBuff       [] byte
	buffer         [] byte
	maxpacket      uint64
	unpackSize     uint64
	unpackIdx      uint64
	initBuffSize   uint64
	totalMaxPacket uint64 
}

func NewPBReceiver(maxMsgSize uint64) (*PBReceiver) {
	receiver := &PBReceiver{}
	//完整数据包大小为head+data
	receiver.totalMaxPacket = maxMsgSize + pb.PBHeaderSize
	doubleTotalPacketSize := receiver.totalMaxPacket*2
	if doubleTotalPacketSize < minBuffSize {
		receiver.initBuffSize = minBuffSize
	}else {
		receiver.initBuffSize = doubleTotalPacketSize
	}
	receiver.buffer = make([]byte,receiver.initBuffSize)
	receiver.recvBuff = receiver.buffer
	receiver.maxpacket = maxMsgSize
	return receiver
}

func (this *PBReceiver) unPack() (interface{},error) {

	if this.unpackSize < pb.PBHeaderSize {
		return nil,nil
	}

	msg,dataLen,err := pb.Decode(this.buffer,this.unpackIdx,this.unpackIdx + this.unpackSize,this.maxpacket)

	if dataLen > 0 {
		this.unpackIdx += dataLen
		this.unpackSize -= dataLen
	}

	return msg,err

}

func (this *PBReceiver) ReceiveAndUnpack(sess kendynet.StreamSession) (interface{},error) {
	var msg interface{}
	var err error
	for {
		msg,err = this.unPack()
		if nil != msg {
			break
		}
		if err == nil {	
			//如果缓冲区小于minSizeRemain字节，重新分配缓冲区
			if len(this.recvBuff) < minSizeRemain {
				buffer := make([]byte,this.initBuffSize)
				if this.unpackSize > 0 {
					//有数据待解包，拷贝到buffer
					copy(buffer,this.buffer[this.unpackIdx:this.unpackIdx+this.unpackSize])
				}
				this.buffer = buffer
				this.recvBuff = buffer[this.unpackSize:]
				this.unpackIdx = 0
			}

			n,err := sess.(*kendynet.StreamSocket).Read(this.recvBuff)
			if err != nil {
				return nil,err
			}

			this.unpackSize += uint64(n) //增加待解包数据
			this.recvBuff = this.recvBuff[n:]
		}else {
			break
		}
	}

	return msg,err
}
