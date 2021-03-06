package timer


import( 
	"time"
	"sync"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/util"
	"runtime"
	"fmt"
)

type TimerID uint64

type timer struct {
	heapIdx    uint32	
	id         TimerID
	expired    time.Time               //到期时间
	eventQue  *kendynet.EventQueue     
	timeout    time.Duration
	repeat     bool                    //是否重复定时器
	callback   func(TimerID)
}

func (this *timer) Less(o util.HeapElement) bool {
	return o.(*timer).expired.After(this.expired)
}

func (this *timer) GetIndex() uint32 {
	return this.heapIdx
}

func (this *timer) SetIndex(idx uint32) {
	this.heapIdx = idx
}


var	(
	idcounter   uint64
	opChan      chan *op
	minheap    *util.MinHeap
	timer_pool  sync.Pool
	op_pool     sync.Pool
	mtx         sync.Mutex
	idTimerMap  map[TimerID]*timer
)

const (
	op_register = 1    //注册定时器
	op_drop     = 2    //丢弃定时器
	op_wakeup   = 3    //唤醒
)

type op struct {
	tt      int32           //操作类型
	data   interface{}
}

func timer_get() *timer {
	t := timer_pool.Get().(*timer)
	idcounter++                        //只在主循环中访问
	t.id = TimerID(idcounter)
	return t
}

func timer_put(t *timer) {
	timer_pool.Put(t)
}


func op_get() *op {
	return op_pool.Get().(*op)
}

func op_put(o *op) {
	op_pool.Put(o)
}

func pcall(callback func(TimerID),id TimerID) {
	defer func(){
		if r := recover(); r != nil {
			buf := make([]byte, 65535)
			l := runtime.Stack(buf, false)
			err := fmt.Errorf("%v: %s", r, buf[:l])
			kendynet.Errorf(util.FormatFileLine("%s\n",err.Error()))
		}			
	}()
	callback(id)
}


func loop() {

	defaultSleepTime := 10 * time.Second
	var t *time.Timer
	var ok bool
	var min util.HeapElement
	for {
		now := time.Now()
		for {
			min = minheap.Min()
			if nil != min && now.After(min.(*timer).expired) {
				t := min.(*timer)
				minheap.PopMin()
				mtx.Lock()
				_,ok = idTimerMap[t.id]
				mtx.Unlock()
				if ok { //ok==false表示已经被调用过DropTimer
					//执行到这里之后,DropTimer将无法终止定时器的下次执行
					if t.repeat {
						//再次注册
						t.expired = time.Now().Add(t.timeout)
						minheap.Insert(t)
					} else {
	 					delete(idTimerMap, t.id)
						timer_put(t)
					}
					if nil == t.eventQue {
						pcall(t.callback,t.id)
					} else {
						t.eventQue.Post(func () {
							pcall(t.callback,t.id)
						})
					}
				}
			} else {
				break
			}
		}

		sleepTime := defaultSleepTime
		if nil != min {
			sleepTime = min.(*timer).expired.Sub(now)
		}

		if nil != t {
			t.Stop()
			t.Reset(sleepTime)
		} else {
			t = time.AfterFunc(sleepTime, func() {
				o := op_get()
				o.tt = op_wakeup
				opChan <- o
			})
		}

		o := <- opChan
		switch (o.tt) {
		case op_register:
			t := o.data.(*timer)

			mtx.Lock()
			if _,ok := idTimerMap[t.id]; ok {
				//确保timer没有被Drop
				minheap.Insert(t)				
			}
			mtx.Unlock()
			break
		case op_drop:
			t := o.data.(*timer)
			minheap.Remove(t)
			timer_put(t)
			break
		default:
			break
		}
		op_put(o)
	}
}


/*
*  timeout:    超时时间
*  repeat:     是否重复定时器
*  eventQue:   如果非nil,callback会被投递到eventQue，否则在定时器主循环中执行
*  返回定时器ID,后面要取消定时器时需要使用这个ID
*/

func newTimer(timeout time.Duration,repeat bool,eventQue *kendynet.EventQueue,callback func(TimerID)) TimerID {
	if nil == callback {
		return 0
	}
	t := timer_get()
	t.timeout  = timeout
	t.expired  = time.Now().Add(timeout)
	t.repeat   = repeat
	t.callback = callback
	t.eventQue = eventQue


	//先插入到idTimerMap中，以允许后面执行DropTimer
	mtx.Lock()
	idTimerMap[t.id] = t
	mtx.Unlock()

	o := op_get()
	o.tt = op_register
	o.data = t
	opChan <- o
	return t.id
}

//一次性定时器
func Once(timeout time.Duration,eventQue *kendynet.EventQueue,callback func(TimerID)) TimerID {
	return newTimer(timeout,false,eventQue,callback)
}

//重复定时器
func Repeat(timeout time.Duration,eventQue *kendynet.EventQueue,callback func(TimerID)) TimerID {
	return newTimer(timeout,true,eventQue,callback)
}

/*
*  终止定时器
*  注意：因为定时器在单独go程序中调度，DropTimer不保证能终止定时器的下次执行（例如定时器马上将要被调度执行，此时在另外
*        一个go程中调用DropTimer），对于重复定时器，可以保证定时器最多在执行一次之后终止。
*/
func DropTimer(id TimerID) {
	mtx.Lock()
	t,ok := idTimerMap[id]
	if ok {
		delete(idTimerMap,id)
	}
	mtx.Unlock()

	if ok {
		o := op_get()
		o.tt = op_drop
		o.data = t
		opChan <- o
	}
}


func init() {
	opChan  = make(chan *op,65536)
	minheap = util.NewMinHeap(65536)
	idTimerMap = map[TimerID]*timer{}
	timer_pool    = sync.Pool{
		New : func() interface{} {
			return &timer{}
		},
	}
	op_pool    = sync.Pool{
		New : func() interface{} {
			return &op{}
		},
	}
	go loop()
}
