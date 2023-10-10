package entities

import (
	"bytes"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pongWait    = 60 * time.Second
	pingPeriod  = 54 * time.Second
	writeWait   = 10 * time.Second
	readMaxSize = 1024
)

var (
	newLine = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	HandshakeTimeout: 3 * time.Second,
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	CheckOrigin:      func(r *http.Request) bool { return true },
	Error:            func(w http.ResponseWriter, r *http.Request, status int, reason error) {},
}

type Client struct {
	send      chan []byte
	hub       *Hub
	conn      *websocket.Conn
	frontName []byte
}

func (c *Client) Read() {
	defer func() {

		c.hub.unregister <- c //不讀取就註銷該client,可能是瀏覽器關閉或網路斷線
		c.conn.Close()
	}()
	c.conn.SetReadLimit(readMaxSize)                 //設置每次讀取前端給的訊息大小,超過會報錯
	c.conn.SetReadDeadline(time.Now().Add(pongWait)) //設定 超時時間

	c.conn.SetPongHandler(func(appData string) error { //當client端 pong 回來 所要做的動作
		if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil { // 設置 等待回應的超時時間
			return err
		}
		return nil
	})

	for {

		_, msg, err := c.conn.ReadMessage() //讀取前端給的message

		if err != nil { //這邊處理例外狀況,例如斷線,瀏覽器關閉等等
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) { //這邊判斷非預期情況
				log.Printf("unexpect websocket error :%s", err.Error())
			} else {
				log.Printf("expect websocket error :%s", err.Error())
			}
			break
		} else {
			msg = bytes.TrimSpace(bytes.Replace(msg, newLine, space, -1))
			if len(c.frontName) == 0 { //frontName沒值代表還沒輸入使用者名稱, 這邊主要是特例
				c.frontName = msg //第一次輸入的內容是使用者名稱,所以要寫到frontName
			} else { //除了第一次來的message是使用者名稱,後面的都是使用者輸入的對話內容,並將對話內容塞入hub中的broadcast channel
				c.hub.broadcast <- bytes.Join([][]byte{c.frontName, msg}, []byte(": ")) //bytes.Join這邊主要是將名稱加上冒號跟內容 e.g. jason: hello
			}
		}

	}

}

func (c *Client) Write() {
	ticker := time.NewTicker(pingPeriod) //設定定時器來做事情
	defer func() {
		ticker.Stop()
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send: //將client端的訊息拿出來
			if !ok { //如果是false 代表client 可能斷線或關閉瀏覽器
				log.Printf("client端讀取send channel 失敗")
				err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}) // 通知client端關閉訊息
				if err != nil {
					log.Printf("WriteMessage for closeMessage error:%s \n", err.Error())
				}
				return
			}
			writer, err := c.conn.NextWriter(websocket.TextMessage) //創建寫入器,方便多次寫入
			if err != nil {
				log.Printf("NextWriter error:%s", err.Error())
				return
			}
			defer func() {
				if err = writer.Close(); err != nil { //使用完記得關閉writer,釋放資源
					log.Printf("write close error:%s \n", err.Error())
					return
				}
			}()
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil { //設定超時時間,這邊是設置10秒,超過就會中斷websocket連線
				log.Printf("SetWriteDeadline error:%s \n", err.Error())
				return
			}
			if _, err = writer.Write(msg); err != nil { //發送訊息到client端,如果超時會回報錯誤
				log.Printf("write msg error :%s\n", err.Error())
				return
			}

			if _, err = writer.Write(newLine); err != nil { //發送換行符號到client端
				log.Printf("write newLine error :%s \n", err.Error())
				return
			}

			msgAmout := len(c.send)         //如果同時send裡面還有其他message,就一併傳給前端,提高效能,查看還有幾個訊息
			for i := 0; i < msgAmout; i++ { //將每個訊息寫入
				if _, err = writer.Write(<-c.send); err != nil {
					log.Printf("write send message error :%s \n", err.Error())
					return
				}
				if _, err = writer.Write(newLine); err != nil {
					log.Printf("write send message error :%s \n", err.Error())
					return
				}
			}

		case <-ticker.C: //計時器,為了定時做某些事情

			if err := c.conn.SetReadDeadline(time.Now().Add(writeWait)); err != nil { //設定超時時間
				log.Printf("SetReadDeadline error: %s", err.Error())
				return
			}

			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil { //ping一下client端看對方是否還在
				log.Printf("pingMessage error:%s", err.Error())
				return
			}

		}
	}

}

func WsServie(hub *Hub, wr http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(wr, r, nil) //升級協議 建立websocket 物件
	if err != nil {
		log.Printf("upgrader error:%s \n", err.Error())
		return
	}
	client := &Client{
		send: make(chan []byte, 256),
		hub:  hub,
		conn: conn,
	}
	client.hub.register <- client //每個打websocket的client端,都註冊到hub的client端

	go client.Read()  //開啟讀取的線程
	go client.Write() //開啟寫內容的線程
}
