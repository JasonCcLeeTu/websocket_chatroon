package entities

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
	}
}

func (h *Hub) Run() {

	for {
		select {
		case client := <-h.register: //註冊client端
			h.clients[client] = true

		case client := <-h.unregister: //註銷client端
			if _, exit := h.clients[client]; exit {
				close(client.send)
				delete(h.clients, client)
			}

		case msg := <-h.broadcast: //broadcast channel 是存放所有client端講的話,

			for client := range h.clients { // 這邊是把某個用戶的話分配到每個client端,這樣每個client端才看的到其他人的訊息
				select {
				case client.send <- msg: //將某個client端的話分配給每個client的send channel,讓各個client端收到某個人發的消息
				default: //會需要default 是因為 當client端的send channel 緩衝滿了 msg塞不進去 所以select 會堵塞 ,若要避免堵塞需要default,當case 的channel都堵塞就會跑去default
					close(client.send)
					delete(h.clients, client)
				}
			}
		}

	}

}
