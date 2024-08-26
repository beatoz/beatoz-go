package web3

import (
	"fmt"
	"github.com/beatoz/beatoz-go/libs/web3/types"
	"github.com/gorilla/websocket"
	"github.com/tendermint/tendermint/libs/json"
	"sync"
)

type Subscriber struct {
	url  string
	conn *websocket.Conn

	query string
	mtx   sync.Mutex
}

func NewSubscriber(url string) (*Subscriber, error) {
	return &Subscriber{
		url: url,
	}, nil
}

func (sub *Subscriber) Start(query string, callback func(*Subscriber, []byte)) error {
	conn, _, err := websocket.DefaultDialer.Dial(sub.url, nil)
	if err != nil {
		return err
	}

	req, err := types.NewRequest(0, "subscribe", query)
	if err != nil {
		return err
	}

	bz, err := json.Marshal(req)
	if err != nil {
		return err
	}

	err = conn.WriteMessage(websocket.TextMessage, bz)
	if err != nil {
		return err
	}

	_, _, err = conn.ReadMessage()
	if err != nil {
		return err
	}

	sub.conn = conn
	sub.query = query

	go receiveRoutine(sub, callback)

	return nil
}

func (sub *Subscriber) Stop() {
	sub.mtx.Lock()
	defer sub.mtx.Unlock()

	if sub.conn == nil {
		return
	}

	req, err := types.NewRequest(1, "unsubscribe", sub.query)
	if err != nil {
		panic(err)
	}

	bz, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}

	err = sub.conn.WriteMessage(websocket.TextMessage, bz)
	if err != nil {
		panic(err)
	}
	sub.query = ""

	_ = sub.conn.Close()
	sub.conn = nil
}

func receiveRoutine(sub *Subscriber, callback func(*Subscriber, []byte)) {
	for {
		if sub.conn == nil {
			break
		}

		ty, msg, err := sub.conn.ReadMessage()
		if err != nil {
			fmt.Println("error", err)
			break
		}

		if ty == websocket.TextMessage {
			resp := &types.JSONRpcResp{}
			if err := json.Unmarshal(msg, resp); err != nil {
				panic(err)
			}

			if resp.Error != nil {
				panic(string(resp.Error))
				break
			}

			if len(resp.Result) > 2 && callback != nil {
				callback(sub, resp.Result)
			}
		} else {
			fmt.Println("ReadMessage", "other type", ty, msg)
		}
	}

	sub.Stop()
}
