package broadcaster

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/go-redis/redis/v7"
	"github.com/notedit/melody"
)

const defaultPubSubChannel = "sockets#pubsub"

// Msg  struct
type Msg struct {
	// the message channel,  like app#room#user  or  app#room
	Channel string `json:"channel,omitempty"`
	Event   string `json:"event,omitempty"`
	// userid, send to the channel except that one
	Exclude uint32                 `json:"exclude,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// Options  config the broadcaster
type Options struct {
	Host         string
	Port         int
	Password     string
	DB           int
	Redis        *redis.Client
	RedisChannel string
}

// BroadCaster
type BroadCaster struct {
	redis        *redis.Client
	channels     map[string]map[string]*melody.Session
	msgCh        chan *Msg
	redisChannel string
	lock         sync.RWMutex
}

func NewBroadCaster(opt *Options) *BroadCaster {
	b := &BroadCaster{}

	if opt.Redis != nil {
		b.redis = opt.Redis
	} else {
		host := "127.0.0.1"
		if opt.Host != "" {
			host = opt.Host
		}

		port := 6379
		if opt.Port > 0 && opt.Port < 65536 {
			port = opt.Port
		}
		redisURI := fmt.Sprintf("%s:%d", host, port)
		b.redis = redis.NewClient(&redis.Options{
			Addr:     redisURI,
			Password: opt.Password,
			DB:       opt.DB,
		})
	}
	if len(opt.RedisChannel) == 0 {
		b.redisChannel = defaultPubSubChannel
	} else {
		b.redisChannel = opt.RedisChannel
	}
	b.channels = make(map[string]map[string]*melody.Session, 0)
	b.msgCh = make(chan *Msg, 100)
	return b
}

// Publish  send msg to the redis pubsub channel
func (b *BroadCaster) Publish(msg *Msg) error {
	if b.redis == nil {
		return errors.New("broadcaster does not have redis client yet")
	}
	msgdata, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	err = b.redis.Publish(b.redisChannel, msgdata).Err()
	return err
}

// Emit to one channel
func (b *BroadCaster) Emit(msg *Msg) {
	b.lock.RLock()
	defer b.lock.RUnlock()

	msgdata, _ := json.Marshal(msg)

	// iterate through each connection in the room
	for _, conn := range b.channels[msg.Channel] {
		if conn.User != msg.Exclude {
			conn.Write(msgdata)
		}
	}
}

// Close  close the redis conn
func (b *BroadCaster) Close() {
	if b.redis != nil {
		b.redis.Close()
	}
}

// Join  one channel
func (b *BroadCaster) Join(channel string, conn *melody.Session) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if _, ok := b.channels[channel]; !ok {
		b.channels[channel] = make(map[string]*melody.Session)
	}

	b.channels[channel][conn.ID] = conn
}

// Leave one channel
func (b *BroadCaster) Leave(channel string, conn *melody.Session) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if conns, ok := b.channels[channel]; ok {
		delete(conns, conn.ID)

		if len(conns) == 0 {
			delete(b.channels, channel)
		}
	}
}

// Channel get message from redis channel
// the redis pubsub will auto reconnect
func (b *BroadCaster) Channel() <-chan *Msg {

	go func() {
		pubsub := b.redis.Subscribe(b.redisChannel)
		ch := pubsub.Channel()

		for mess := range ch {
			var msg Msg
			err := json.Unmarshal([]byte(mess.Payload), &msg)
			if err != nil {
				fmt.Println(err)
				continue
			}
			b.msgCh <- &msg
		}
	}()
	return b.msgCh
}
