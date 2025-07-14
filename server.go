package main

import (
	"fmt"
	"net"
	"strconv"
)

type Server struct {
	Ip   string
	Port int
}

// 创建一个 server 的接口
func NewServer(ip string, port int) *Server {
	return &Server{Ip: ip, Port: port}
}

func (s *Server) Handler(conn net.Conn) {
	// 当前连接的业务
	fmt.Println("连接建立成功")
}

// 启动服务器的接口
func (s *Server) Start() {
	// socket listen
	listener, err := net.Listen("tcp", s.Ip+":"+strconv.Itoa(s.Port))
	if err != nil {
		fmt.Println("net.Listen err:", err)
		return
	}
	// close listen socket
	defer listener.Close()

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
