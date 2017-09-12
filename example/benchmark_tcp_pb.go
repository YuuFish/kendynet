package main

import(
	"time"
	"sync/atomic"
	"strconv"
	"fmt"
	"os"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/tcp"
	"github.com/sniperHW/kendynet/protocal/protocal_stream_socket"	
	"github.com/sniperHW/kendynet/util/pb"	
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/golang/protobuf/proto"
)

func server(service string) {
	clientcount := int32(0)
	packetcount := int32(0)



	go func() {
		for {
			time.Sleep(time.Second)
			tmp := atomic.LoadInt32(&packetcount)
			atomic.StoreInt32(&packetcount,0)
			fmt.Printf("clientcount:%d,packetcount:%d\n",clientcount,tmp)			
		}
	}()

	server,err := tcp.NewServer("tcp4",service)
	if server != nil {
		fmt.Printf("server running on:%s\n",service)
		err = server.Start(func(session kendynet.StreamSession) {
			atomic.AddInt32(&clientcount,1)
			session.SetEncoder(protocal_stream_socket.NewPbEncoder(4096))
			session.SetReceiver(protocal_stream_socket.NewPBReceiver(4096))
			session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
				fmt.Printf("server client close:%s\n",reason)
				atomic.AddInt32(&clientcount,-1)
			})
			session.SetEventCallBack(func (event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(),0)
				} else {
					//fmt.Printf("server on msg\n")
					atomic.AddInt32(&packetcount,int32(1))
					event.Session.Send(event.Data.(proto.Message))
				}
			})
			session.Start()
		})

		if nil != err {
			fmt.Printf("TcpServer start failed %s\n",err)			
		}

	} else {
		fmt.Printf("NewTcpServer failed %s\n",err)
	}
}

func client(service string,count int) {
	
	client,err := tcp.NewClient("tcp4",service)

	if err != nil {
		fmt.Printf("NewTcpClient failed:%s\n",err.Error())
		return
	}



	for i := 0; i < count ; i++ {
		session,_,err := client.Dial()
		if err != nil {
			fmt.Printf("Dial error:%s\n",err.Error())
		} else {
			session.SetEncoder(protocal_stream_socket.NewPbEncoder(4096))
			session.SetReceiver(protocal_stream_socket.NewPBReceiver(4096))
			session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
				fmt.Printf("client client close:%s\n",reason)
			})
			session.SetEventCallBack(func (event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(),0)
				} else {
					//fmt.Printf("client on msg\n")
					event.Session.Send(event.Data.(proto.Message))
				}
			})
			session.Start()
			//send the first messge
			o := &testproto.Test{}
			o.A = proto.String("hello")
			o.B = proto.Int32(17)
			session.Send(o)
		}
	}
}


func main(){

	if len(os.Args) < 3 {
		fmt.Printf("usage ./pingpong [server|client|both] ip:port clientcount\n")
		return
	}


	mode := os.Args[1]

	if !(mode == "server" || mode == "client" || mode == "both") {
		fmt.Printf("usage ./pingpong [server|client|both] ip:port clientcount\n")
		return
	}

	pb.Register(testproto.Test{})

	service := os.Args[2]

	sigStop := make(chan bool)

	if mode == "server" || mode == "both" {
		go server(service)
	}

	if mode == "client" || mode == "both" {
		if len(os.Args) < 4 {
			fmt.Printf("usage ./pingpong [server|client|both] ip:port clientcount\n")
			return
		}
		connectioncount,err := strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Printf(err.Error())
			return
		}
		//让服务器先运行
		time.Sleep(10000000)
		go client(service,connectioncount)

	}

	_,_ = <- sigStop

	return

}


