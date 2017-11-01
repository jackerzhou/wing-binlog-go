package subscribe

import "fmt"
import (
	"library/debug"
	"library/base"
	"runtime"
)

type Tcp struct {
	base.Subscribe
	queue chan map[string] interface{}
}

func (r *Tcp) Init() {
	r.queue = 	make(chan map[string] interface{}, base.MAX_EVENT_QUEUE_LENGTH)
	//to := time.NewTimer(time.Second*3)
	cpu := runtime.NumCPU()
	for i := 0; i < cpu; i ++ {
		go func() {
			for {
				select {
				case body := <-r.queue:
					for k,v := range body  {
						debug.Print("tcp---", k, v)
					}

				//case <-to.C://time.After(time.Second*3):
				//	Log("发送超时...")
				}
			}
		} ()
	}
}

func (r *Tcp) OnChange(data map[string] interface{}) {
	r.queue <- data
	fmt.Println("tcp", data)
}

func (r *Tcp) Free() {
	close(r.queue)
}