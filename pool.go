package objpool

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var EMPTYERROR = errors.New("again")
var TIMEOUTERROR = errors.New("block timeout")
var CLOSEDERROR = errors.New("ObjectPool closed")

type Objecter interface {
	Destory()     //销毁对象
	Active() bool //判断对象是否活跃，如不活跃则销毁
}

type ObjectPool struct {
	New      func(args interface{}) (Objecter, error)
	Args     interface{}
	poolchan chan Objecter
	normaln  int32
	topn     int32
	cursize  int32 //实际持有数量，包括Get出去的
	rwMutex  sync.RWMutex
}

//normaln, topn分别代表常驻对象数，和顶峰对象数
//topn - normaln 的范围内，对象会在timeout后调用Destory()
func NewObjectPool(normaln, topn int32, timeout time.Duration) (*ObjectPool, error) {
	objpool := &ObjectPool{
		poolchan: make(chan Objecter, topn),
		normaln:  normaln,
		topn:     topn,
	}
	if timeout == 0 {
		timeout = 30
	}
	go objpool.timeoutWorker(timeout)
	return objpool, nil
}

//获取池对象，block判断是否需要阻塞
//阻塞时,timeout生效，timeout=0则一直阻塞
func (this *ObjectPool) Get(block bool, timeout time.Duration) (Objecter, error) {
	//判断当前数是否达到topn,否则创建对象
	if block {
		return this.getBlock(timeout)
	}
	return this.getNoBlock()
}

//回收池对象
func (this *ObjectPool) Recycle(obj Objecter) {
	defer func() {
		recover()
	}()
	if !obj.Active() {
		obj.Destory()
		atomic.AddInt32(&this.cursize, -1)
		return
	}
	this.poolchan <- obj

}

func (this *ObjectPool) Close() {
	//关闭poolchan
	atomic.AddInt32(&this.cursize, this.topn)
	close(this.poolchan)
	//推出所有对象
	for _ = range this.poolchan {

	}
}

///////////private function//////////////////////////////////////////
func (this *ObjectPool) getNoBlock() (Objecter, error) {
	select {
	case obj := <-this.poolchan:
		return obj, nil
	default:
	}
	//判断是否可用增加对象
	this.rwMutex.Lock()
	defer this.rwMutex.Unlock()
	if this.cursize < this.topn {
		//新建对象
		obj, err := this.New(this.Args)
		if err != nil {
			return nil, err
		}
		atomic.AddInt32(&this.cursize, 1)
		return obj, nil
	}
	return nil, EMPTYERROR
}

func (this *ObjectPool) getBlock(timeout time.Duration) (Objecter, error) {
	this.rwMutex.Lock()
	if len(this.poolchan) == 0 && this.cursize < this.topn {
		//创建新创建
		obj, err := this.New(this.Args)
		if err != nil {
			//
		} else {
			atomic.AddInt32(&this.cursize, 1)
			this.rwMutex.Unlock()
			return obj, nil
		}
	}
	this.rwMutex.Unlock()

	if timeout != 0 {
		select {
		case obj, ok := <-this.poolchan:
			if !ok {
				return nil, CLOSEDERROR
			}
			return obj, nil
		case <-time.After(timeout * time.Second):
			return nil, TIMEOUTERROR
		}
	}

	obj, ok := <-this.poolchan
	if !ok {
		return nil, CLOSEDERROR
	}
	return obj, nil
}

//定时删除过期对象
//采用缓慢下降过程
func (this *ObjectPool) timeoutWorker(interval time.Duration) {
	for {
		time.Sleep(interval * time.Second)
		if len(this.poolchan) > int(this.normaln) {
			obj, ok := <-this.poolchan
			if !ok {
				return
			}
			obj.Destory()
			atomic.AddInt32(&this.cursize, -1)
		}
		this.Info()
	}
}

func (this *ObjectPool) Info() {
	fmt.Println("cursize:", this.cursize)
}
