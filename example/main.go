package main

import (
	"fmt"
	"math/rand"

	"github.com/gin-gonic/gin"
	"github.com/notedit/broadcaster"
	"github.com/notedit/melody"
)

var broadcast *broadcaster.BroadCaster

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func publishMessage(c *gin.Context) {

	var data struct {
		Event string                 `json:"event"`
		Data  map[string]interface{} `json:"data"`
	}

	if err := c.ShouldBind(&data); err != nil {
		c.JSON(200, gin.H{"s": 10001, "e": err})
		return
	}

	msg := &broadcaster.Msg{
		Channel: "channel1",
		Event:   data.Event,
		Data:    data.Data,
		Exclude: 100,
	}

	broadcast.Publish(msg)

	c.JSON(200, gin.H{"s": 10000})

}

func handleMessage() {

	messages := broadcast.Channel()

	for mess := range messages {
		fmt.Println("Message ", mess)
		broadcast.Emit(mess)
	}
}

func index(c *gin.Context) {
	fmt.Println("hello world")
	c.HTML(200, "index.html", gin.H{})
}

func main() {

	opt := &broadcaster.Options{}
	broadcast = broadcaster.NewBroadCaster(opt)

	go handleMessage()

	r := gin.Default()
	m := melody.New()

	r.GET("/test", func(c *gin.Context) {
		c.String(200, "Hello World")
	})

	r.GET("/ws", func(c *gin.Context) {
		m.HandleRequest(c.Writer, c.Request)
	})

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		fmt.Println("HandleMessage", msg)
	})

	m.HandleConnect(func(s *melody.Session) {
		fmt.Println("HandleConnect")
		channel := s.Request.FormValue("channel")
		s.ID = randSeq(10)
		s.User = rand.Uint32()
		broadcast.Join(channel, s)

		msg := &broadcaster.Msg{
			Channel: channel,
			Event:   "connect",
			Data:    map[string]interface{}{},
			Exclude: s.User,
		}

		broadcast.Publish(msg)
	})

	m.HandleDisconnect(func(s *melody.Session) {
		fmt.Println("HandleDisconnect")
		channel := s.Request.FormValue("channel")
		broadcast.Leave(channel, s)

		msg := &broadcaster.Msg{
			Channel: channel,
			Event:   "disconnect",
			Data:    map[string]interface{}{},
			Exclude: s.User,
		}

		broadcast.Publish(msg)
	})

	m.HandlePong(func(s *melody.Session) {
		fmt.Println("ping response")
	})

	r.POST("/publish", publishMessage)

	r.LoadHTMLFiles("./index.html")
	r.GET("/", index)

	r.Run(":8080")
}
