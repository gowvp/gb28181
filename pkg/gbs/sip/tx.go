package sip

import (
	"net/http"
	"sync"
	"time"
	"unsafe"
)

const txIdleTimeout = 20 * time.Second

var activeTX *transacionts

type transacionts struct {
	txs map[string]*Transaction
	rwm *sync.RWMutex
}

func (txs *transacionts) newTX(key string, conn Connection) *Transaction {
	tx := NewTransaction(key, conn)
	txs.rwm.Lock()
	txs.txs[key] = tx
	txs.rwm.Unlock()
	return tx
}

func (txs *transacionts) getTX(key string) *Transaction {
	txs.rwm.RLock()
	tx, ok := txs.txs[key]
	if !ok {
		tx = nil
	}
	txs.rwm.RUnlock()
	return tx
}

func (txs *transacionts) rmTX(tx *Transaction) {
	txs.rwm.Lock()
	delete(txs.txs, tx.key)
	txs.rwm.Unlock()
}

// Transaction Transaction
type Transaction struct {
	conn   Connection
	key    string
	resp   chan *Response
	active chan int
	once   sync.Once
}

// NewTransaction NewTransaction
func NewTransaction(key string, conn Connection) *Transaction {
	// logrus.Traceln("new tx", key, time.Now().Format("2006-01-02 15:04:05"))
	tx := &Transaction{conn: conn, key: key, resp: make(chan *Response, 10), active: make(chan int, 1)}
	go tx.watch()
	return tx
}

// Key Key
func (tx *Transaction) Key() string {
	return tx.key
}

func (tx *Transaction) watch() {
	timer := time.NewTimer(txIdleTimeout)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-tx.active:
			// Close() 关闭 active 后必须退出；否则已关闭 channel 会让 select
			// 立刻命中，并在循环里反复创建 time.After，把 CPU 打满。
			if !ok {
				return
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(txIdleTimeout)
		case <-timer.C:
			tx.Close()
			return
		}
	}
}

// GetResponse GetResponse
func (tx *Transaction) GetResponse() *Response {
	for {
		res := <-tx.resp
		if res == nil {
			return res
		}
		tx.active <- 2
		// logrus.Traceln("response tx", tx.key, time.Now().Format("2006-01-02 15:04:05"))
		if res.StatusCode() == http.StatusContinue || res.statusCode == http.StatusSwitchingProtocols {
			// Trying and Dialog Establishement 等待下一个返回
			continue
		}
		return res
	}
}

// Close Close
func (tx *Transaction) Close() {
	tx.once.Do(func() {
		if activeTX != nil {
			activeTX.rmTX(tx)
		}
		close(tx.resp)
		close(tx.active)
	})
}

// Response Response
func (tx *Transaction) receiveResponse(msg *Response) {
	defer func() {
		if r := recover(); r != nil {
			// logrus.Errorln("send to closed channel, txkey:", tx.key, "message: \n", msg.String())
		}
	}()
	// logrus.Traceln("receiveResponse tx", tx.Key(), time.Now().Format("2006-01-02 15:04:05"))
	tx.resp <- msg
	tx.active <- 1
}

// Respond Respond
func (tx *Transaction) Respond(res *Response) error {
	// logrus.Traceln("send response,to:", res.dest.String(), "txkey:", tx.key, "message: \n", res.String())
	_, err := tx.conn.WriteTo([]byte(res.String()), res.dest)
	return err
}

// Request Request
func (tx *Transaction) Request(req *Request) error {
	str := req.String()
	s := unsafe.Slice(unsafe.StringData(str), len(str))
	// logrus.Traceln("send request,to:", req.dest.String(), "txkey:", tx.key, "message: \n", req.String())
	_, err := tx.conn.WriteTo(s, req.dest)
	return err
}

func getTXKey(msg Message) (key string) {
	callid, ok := msg.CallID()
	if ok {
		key = callid.String()
	} else {
		key = RandString(10)
	}
	return
}
