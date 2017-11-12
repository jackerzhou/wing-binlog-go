package services

import (
	"fmt"
	"github.com/go-martini/martini"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"time"
	"sync/atomic"
)

type websocket_client_node struct {
	conn *websocket.Conn     // 客户端连接进来的资源句柄
	is_connected bool        // 是否还连接着 true 表示正常 false表示已断开
	send_queue chan []byte   // 发送channel
	send_failure_times int64 // 发送失败次数
	mode int                 // broadcast = 1 weight = 2 支持两种方式，广播和权重
	weight int               // 权重 0 - 100
	group string             // 所属分组
	recv_buf []byte          // 读缓冲区
	recv_bytes int           // 收到的待处理字节数量
	connect_time int64       // 连接成功的时间戳
	send_times int64         // 发送次数，用来计算负载均衡，如果 mode == 2
}

type WebSocketService struct {
	Ip string                             // 监听ip
	Port int                              // 监听端口
	recv_times int64                      // 收到消息的次数
	send_times int64                      // 发送消息的次数
	send_failure_times int64              // 发送失败的次数
	send_queue chan []byte                // 发送队列-广播
	lock *sync.Mutex                      // 互斥锁，修改资源时锁定
	groups map[string][]*websocket_client_node
										  // 客户端分组，现在支持两种分组，广播组合负载均衡组
	groups_mode map[string] int           // 分组的模式 1，2 广播还是复载均衡
	clients_count int32                   // 成功连接（已经进入分组）的客户端数量
}

func NewWebSocketService(ip string, port int, config *TcpConfig) *WebSocketService {
	ws := &WebSocketService {
		Ip                 : ip,
		Port               : port,
		clients_count      : int32(0),
		lock               : new(sync.Mutex),
		send_queue         : make(chan []byte, TCP_MAX_SEND_QUEUE),
		groups             : make(map[string][]*websocket_client_node),
		groups_mode        : make(map[string] int),
		recv_times         : 0,
		send_times         : 0,
		send_failure_times : 0,
	}

	for _, v := range config.Groups {
		var con [TCP_DEFAULT_CLIENT_SIZE]*websocket_client_node
		ws.groups[v.Name]      = con[:0]
		ws.groups_mode[v.Name] = v.Mode
	}

	return ws
}

// 对外的广播发送接口
func (ws *WebSocketService) SendAll(msg []byte) bool {
	cc := atomic.LoadInt32(&ws.clients_count)
	if cc <= 0 {
		return false
	}
	if len(ws.send_queue) >= cap(ws.send_queue) {
		log.Println("websocket发送缓冲区满...")
		return false
	}
	ws.send_queue <- ws.pack(CMD_EVENT, string(msg))
	return true
}

func (ws *WebSocketService) broadcast() {
	to := time.NewTimer(time.Second*1)
	for {
		select {
		case  msg := <-ws.send_queue:
			ws.lock.Lock()
			for group_name, clients := range ws.groups {
				// 如果分组里面没有客户端连接，跳过
				if len(clients) <= 0 {
					continue
				}
				// 分组的模式
				mode := ws.groups_mode[group_name]
				// 如果不等于权重，即广播模式
				if mode != MODEL_WEIGHT {
					for _, conn := range clients {
						if !conn.is_connected {
							continue
						}
						log.Println("发送广播消息")
						conn.send_queue <- msg
					}
				} else {
					// 负载均衡模式
					// todo 根据已经send_times的次数负载均衡
					target := clients[0]
					//将发送次数/权重 作为负载基数，每次选择最小的发送
					js := atomic.LoadInt64(&target.send_times)/int64(target.weight)

					for _, conn := range clients {
						stimes := atomic.LoadInt64(&conn.send_times)
						//conn.send_queue <- msg
						if stimes == 0 {
							//优先发送没有发过的
							target = conn
							break
						}
						_js := stimes/int64(conn.weight)
						if _js < js {
							js = _js
							target = conn
						}
					}
					log.Println("发送权重消息，", (*target.conn).RemoteAddr().String())
					target.send_queue <- msg
				}
			}
			ws.lock.Unlock()
		case <-to.C://time.After(time.Second*3):
		}
	}
}

// 打包tcp响应包 格式为 [包长度-2字节，大端序][指令-2字节][内容]
func (ws *WebSocketService) pack(cmd int, msg string) []byte {
	m := []byte(msg)
	l := len(m)
	r := make([]byte, l + 6)

	cl := l + 2

	r[0] = byte(cl)
	r[1] = byte(cl >> 8)
	r[2] = byte(cl >> 16)
	r[3] = byte(cl >> 32)

	r[4] = byte(cmd)
	r[5] = byte(cmd >> 8)
	copy(r[6:], m)

	return r
}

func (ws *WebSocketService) onClose(conn *websocket_client_node) {
	if conn.group == "" {
		ws.lock.Lock()
		conn.is_connected = false
		close(conn.send_queue)
		ws.lock.Unlock()
		return
	}
	//移除conn
	//查实查找位置
	ws.lock.Lock()
	close(conn.send_queue)
	for index, con := range ws.groups[conn.group] {
		if con.conn == conn.conn {
			con.is_connected = false
			ws.groups[conn.group] = append(ws.groups[conn.group][:index], ws.groups[conn.group][index+1:]...)
			break
		}
	}
	ws.lock.Unlock()
	atomic.AddInt32(&ws.clients_count, int32(-1))
	log.Println("当前连输的客户端：", len(ws.groups[conn.group]), ws.groups[conn.group])
}

func (ws *WebSocketService) clientSendService(node *websocket_client_node) {
	to := time.NewTimer(time.Second*1)
	for {
		if !node.is_connected {
			log.Println("ws-clientSendService退出")
			break
		}

		select {
		case  msg := <-node.send_queue:
			(*node.conn).SetWriteDeadline(time.Now().Add(time.Second*1))
			//size, err := (*node.conn).Write(msg)
			err := (*node.conn).WriteMessage(1, msg)

			atomic.AddInt64(&node.send_times, int64(1))

			if (err != nil) {
				atomic.AddInt64(&ws.send_failure_times, int64(1))
				atomic.AddInt64(&node.send_failure_times, int64(1))

				log.Println("ws-失败次数：", ws.send_failure_times, node.conn, node.send_failure_times)
			}
		case <-to.C://time.After(time.Second*3):
		//log.Println("发送超时...", tcp)
		}
	}
}

func (ws *WebSocketService) onConnect(conn *websocket.Conn) {

	log.Println("新的连接：",conn.RemoteAddr().String())
	cnode := &websocket_client_node {
		conn               : conn,
		is_connected       : true,
		send_queue         : make(chan []byte, TCP_MAX_SEND_QUEUE),
		send_failure_times : 0,
		weight             : 0,
		mode               : MODEL_BROADCAST,
		connect_time       : time.Now().Unix(),
		send_times         : int64(0),
		recv_buf           : make([]byte, TCP_RECV_DEFAULT_SIZE),
		recv_bytes         : 0,
		group              : "",
	}


	go ws.clientSendService(cnode)
	// 设定3秒超时，如果添加到分组成功，超时限制将被清除
	conn.SetReadDeadline(time.Now().Add(time.Second*3))
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println(conn.RemoteAddr().String(), "连接发生错误: ", err)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			ws.onClose(cnode)
			conn.Close();
			return
		}
		log.Println("收到websocket消息：", string(message))

		size := len(message)
		atomic.AddInt64(&ws.recv_times, int64(1))
		cnode.recv_bytes += size
		ws.onMessage(cnode, message, size)
	}
}

// 收到消息回调函数
func (ws *WebSocketService) onMessage(conn *websocket_client_node, msg []byte, size int) {
	conn.recv_buf = append(conn.recv_buf[:conn.recv_bytes - size], msg[0:size]...)

	for {
		clen := len(conn.recv_buf)
		if clen < 6 {
			return
		} else if clen > TCP_RECV_DEFAULT_SIZE {
			// 清除所有的读缓存，防止发送的脏数据不断的累计
			conn.recv_buf = make([]byte, TCP_RECV_DEFAULT_SIZE)
			log.Println("新建缓冲区")
			return
		}

		//4字节长度
		content_len := int(conn.recv_buf[0]) +
			int(conn.recv_buf[1] << 8) +
			int(conn.recv_buf[2] << 16) +
			int(conn.recv_buf[3] << 32)

		//2字节 command
		cmd := int(conn.recv_buf[4]) + int(conn.recv_buf[5] << 8)

		//log.Println("content：", conn.recv_buf)
		//log.Println("content_len：", content_len)
		//log.Println("cmd：", cmd)
		switch cmd {
		case CMD_SET_PRO:
			log.Println("收到注册分组消息")
			if len(conn.recv_buf) < 10 {
				return
			}
			//4字节 weight
			weight := int(conn.recv_buf[6]) +
				int(conn.recv_buf[7] << 8) +
				int(conn.recv_buf[8] << 16) +
				int(conn.recv_buf[9] << 32)

			//log.Println("weight：", weight)
			if weight < 0 || weight > 100 {
				conn.send_queue <- ws.pack(CMD_ERROR, fmt.Sprintf("不支持的权重值：%d，请设置为0-100之间", weight))
				return
			}

			//内容长度+4字节的前缀（存放内容长度的数值）
			group := string(conn.recv_buf[10:content_len + 4])
			//log.Println("group：", group)

			ws.lock.Lock()
			is_find := false
			for g, _ := range ws.groups {
				//log.Println(g, len(g), ">" + g + "<", len(group), ">" + group + "<")
				if g == group {
					is_find = true
					break
				}
			}
			if !is_find {
				conn.send_queue <- ws.pack(CMD_ERROR, fmt.Sprintf("组不存在：%s", group))
				ws.lock.Unlock()
				return
			}

			(*conn.conn).SetReadDeadline(time.Time{})
			conn.send_queue <- ws.pack(CMD_SET_PRO, "ok")

			conn.group  = group
			conn.mode   = ws.groups_mode[group]
			conn.weight = weight

			ws.groups[group] = append(ws.groups[group], conn)

			if conn.mode == MODEL_WEIGHT {
				//weight 合理性格式化，保证所有的weight的和是100
				all_weight := 0
				for _, _conn := range ws.groups[group] {
					w := _conn.weight
					if w <= 0 {
						w = 100
					}
					all_weight += w
				}

				gl := len(ws.groups[group])
				yg := 0
				for k, _conn := range ws.groups[group] {
					if k == gl - 1 {
						_conn.weight = 100 - yg
					} else {
						_conn.weight = int(_conn.weight * 100 / all_weight)
						yg += _conn.weight
					}
				}
			}
			atomic.AddInt32(&ws.clients_count, int32(1))
			ws.lock.Unlock()

		case CMD_TICK:
			//log.Println("收到心跳消息")
			conn.send_queue <- ws.pack(CMD_OK, "ok")
		//心跳包
		default:
			conn.send_queue <- ws.pack(CMD_ERROR, fmt.Sprintf("不支持的指令：%d", cmd))
		}

		//数据移动
		//log.Println(content_len + 4, conn.recv_bytes)
		conn.recv_buf = append(conn.recv_buf[:0], conn.recv_buf[content_len + 4:conn.recv_bytes]...)
		conn.recv_bytes = conn.recv_bytes - content_len - 4
		//log.Println("移动后的数据：", conn.recv_bytes, len(conn.recv_buf), string(conn.recv_buf))
	}
}

func (ws *WebSocketService) Start() {

	go ws.broadcast()

	m := martini.Classic()

	m.Get("/", func(res http.ResponseWriter, req *http.Request) {
		// res and req are injected by Martini

		u := websocket.Upgrader{ReadBufferSize: TCP_DEFAULT_READ_BUFFER_SIZE,
			WriteBufferSize: TCP_DEFAULT_WRITE_BUFFER_SIZE}
		u.Error = func(w http.ResponseWriter, r *http.Request, status int, reason error) {
			log.Println(w, r, status, reason)
			// don't return errors to maintain backwards compatibility
		}
		u.CheckOrigin = func(r *http.Request) bool {
			// allow all connections by default
			return true
		}
		conn, err := u.Upgrade(res, req, nil)

		if err != nil {
			log.Println(err)
			return
		}

		log.Println("新的连接：" + conn.RemoteAddr().String())
		go ws.onConnect(conn)
	})

	dns := fmt.Sprintf("%s:%d", ws.Ip, ws.Port)
	m.RunOnAddr(dns)
}