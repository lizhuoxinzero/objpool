package objpool

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

var id int32

type testobj struct {
	Id int32
}

func (this *testobj) Destory() {
	fmt.Println("Destory, id:", this.Id)
}
func (this *testobj) Active() bool {
	return true
}

func Create(args interface{}) (Objecter, error) {
	cid := atomic.AddInt32(&id, 1)
	fmt.Println("Create id:", cid)
	obj := Objecter(&testobj{cid})
	return obj, nil
}

func Worker(wid int, objpool *ObjectPool, quitchan chan int) {
	for i := 0; i < 10000; i++ {
		obj, errg := objpool.Get(true, 5)
		if errg != nil {
			fmt.Println(errg)
		}
		fmt.Println(obj.(*testobj).Id)
		time.Sleep(10 * time.Microsecond)
		objpool.Recycle(obj)
	}
	quitchan <- 1
}

var workernum = 10

func TestNew(t *testing.T) {
	objpool, err := NewObjectPool(4, 8, 10)
	if err != nil {
		fmt.Println("NewObjectPool error:", err)
	}
	quitchan := make(chan int, workernum)

	objpool.New = Create
	for i := 0; i < workernum; i++ {
		go Worker(i, objpool, quitchan)
	}
	for i := 0; i < workernum; i++ {
		<-quitchan
	}
	fmt.Println("out now")
	objpool.Info()
	//	time.Sleep(60 * time.Second)
}
