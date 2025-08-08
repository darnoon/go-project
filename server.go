package main

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type Server struct {
	Ip   string
	Port int

	// 在线用户的列表
	OnlineMap map[string]*User
	mapLock   sync.RWMutex

	// 消息广播的channel
	Message chan string
}

// 创建一个 server 的接口
func NewServer(ip string, port int) *Server {
	server := &Server{
		Ip:        ip,
		Port:      port,
		OnlineMap: make(map[string]*User),
		Message:   make(chan string),
	}

	return server
}

// 监听 Message 广播消息 channel 的 goroutine，一旦有消息就发送给全部的在线 User
func (this *Server) ListenMessager() {
	for {
		msg := <-this.Message

		// 将 msg 发送给全部的在线 User
		// 首先快速复制出所有 channel，防止锁释放不及时
		this.mapLock.RLock()
		targets := make([]chan string, 0, len(this.OnlineMap))
		for _, user := range this.OnlineMap {
			targets = append(targets, user.C)
		}
		this.mapLock.RUnlock()

		for _, userChan := range targets {
			// 使用临时变量，避免闭包问题
			ch := userChan
			go func() {
				// 使用 select 结构实现带超时的非阻塞发送
				select {
				case ch <- msg:
					// 消息成功发送
				case <-time.After(100 * time.Millisecond): // 比如设置 100 毫秒的超时
					// 超时了，意味着对方 goroutine 可能卡住了
					// 我们可以选择记录日志，或者干脆放弃这次发送
					fmt.Println("发送消息超时，用户可能已拥堵。")
				}
			}()
		}
	}
}

// 启动服务器的接口
func (s *Server) Start() {
	// socket listen
	address := fmt.Sprintf("%s:%d", s.Ip, s.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("net.Listen err:", err)
		return
	}
	// close listen socket
	defer listener.Close()

	// 启动监听 Message 的 goroutine
	go s.ListenMessager()

	for {
		// accept
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("net.Accept err:", err)
			continue
		}
		// do handler
		go s.Handler(conn)
	}
}

// 建立连接后的业务
func (s *Server) Handler(conn net.Conn) {
	// 当前连接的业务
	// fmt.Println("连接建立成功")
	user := NewUser(conn, s)

	// 上线广播
	user.Online()

	// 监听用户是否活跃的 channel
	isLive := make(chan bool)

	// 接受客户端发送的消息
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n == 0 {
				user.Offline()
				return
			}
			if err != nil && err != io.EOF {
				fmt.Println("Conn Read err:", err)
				return
			}
			// 提取用户的消息(去除'\n')
			msg := string(buf[:n-1])
			// 用户针对msg进行消息处理
			user.DoMessage(msg)
			// 用户的任意消息，代表当前用户是一个活跃的
			isLive <- true
		}
	}()

	timer := time.NewTimer(600 * time.Second)
	// 当前 handler 阻塞
	for {
		select {
		case <-isLive:
			// 收到心跳信号
			// 重置定时器，让它重新开始倒计时
			// 如果 timer 已经触发但还没被接收，Reset 会返回 false。
			// 为了安全，先尝试停止它。
			if !timer.Stop() {
				// 如果 Stop 返回 false，说明 timer 已经触发并且其 channel 中有值了，
				// 为了防止下一次 select 读到这个旧的值，需要清空 channel。
				<-timer.C
			}
			timer.Reset(10 * time.Second)
		case <-timer.C:
			// 超时，timer.C channel 收到了值
			user.SendMsg("过长时间未活跃被移出聊天")
			// 销毁资源
			close(user.C)
			// 关闭连接
			conn.Close()
			// 退出当前 handler
			return
		}
	}

	// 当前 handler 阻塞
	select {}
}

// 广播消息
func (this *Server) BroadCast(user *User, msg string) {
	sendMsg := "[" + user.Addr + "]" + user.Name + ":" + msg

	this.Message <- sendMsg
}
