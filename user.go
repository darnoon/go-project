package main

import (
	"net"
	"strings"
)

type User struct {
	Name string
	Addr string
	C    chan string
	conn net.Conn

	server *Server
}

// 创建一个用户的 API
func NewUser(conn net.Conn, server *Server) *User {
	userAddr := conn.RemoteAddr().String()

	user := &User{
		Name:   userAddr,
		Addr:   userAddr,
		C:      make(chan string),
		conn:   conn,
		server: server,
	}

	// 主 routine -> user channel -> 客户端
	go user.ListenMessage()

	return user
}

// 用户的上线业务
func (this *User) Online() {

	// 用户上线,将用户加入到onlineMap中
	this.server.mapLock.Lock()
	this.server.OnlineMap[this.Name] = this
	this.server.mapLock.Unlock()

	// 广播当前用户上线消息
	this.server.BroadCast(this, "已上线")
}

// 用户的下线业务
func (this *User) Offline() {

	// 用户下线,将用户从 onlineMap 中删除
	this.server.mapLock.Lock()
	delete(this.server.OnlineMap, this.Name)
	this.server.mapLock.Unlock()

	// 广播当前用户上线消息
	this.server.BroadCast(this, "已下线")
}

// 用户处理消息的业务
func (this *User) DoMessage(msg string) {
	if msg == "who" {
		// 查询当前在线用户都有哪些

		this.server.mapLock.RLock()
		for _, user := range this.server.OnlineMap {
			onlineMsg := "[" + user.Addr + "]" + user.Name + ":" + "在线...\n"
			this.SendMsg(onlineMsg)
		}
		this.server.mapLock.RUnlock()

	} else if len(msg) > 7 && msg[:7] == "rename|" {
		// 消息格式: rename|张三
		// 安全地解析输入
		parts := strings.SplitN(msg, "|", 2) // 最多切成2部分
		if len(parts) != 2 || parts[1] == "" {
			this.SendMsg("改名格式错误，请使用 'rename|新用户名' 格式，且新用户名不能为空。\n")
			return // 提前返回，不再执行后续逻辑
		}
		newName := parts[1]

		// 判断name是否存在
		this.server.mapLock.Lock()
		defer this.server.mapLock.Unlock()
		_, ok := this.server.OnlineMap[newName]
		if ok {
			this.SendMsg("当前用户名被使用\n")
		} else {
			delete(this.server.OnlineMap, this.Name)
			this.server.OnlineMap[newName] = this
			this.Name = newName
			this.SendMsg("您已经更新用户名:" + this.Name + "\n")
		}
	} else if len(msg) > 4 && msg[:3] == "to|" {
		// 消息格式:  to|张三|消息内容
		// 1 获取对方的用户名
		parts := strings.SplitN(msg, "|", 3)
		if len(parts) != 3 {
			this.SendMsg("私聊格式错误，请使用 \"to|用户名|消息内容\" 格式。\n")
			return
		}
		remoteName := parts[1]
		content := parts[2]
		if remoteName == "" {
			this.SendMsg("私聊格式错误：接收方用户名不能为空。\n")
			return
		}
		if content == "" {
			this.SendMsg("私聊消息内容不能为空。\n")
			return
		}
		// 2 根据用户名 得到对方User对象
		this.server.mapLock.RLock()
		remoteUser, ok := this.server.OnlineMap[remoteName]
		this.server.mapLock.RUnlock()
		if !ok {
			this.SendMsg("用户 " + remoteName + " 不存在或不在线。\n")
			return
		}
		// 3 消息内容通过对方的User对象将消息内容发送过去
		remoteUser.SendMsg("[" + this.Name + "] 对您说: " + content)
	} else {
		this.server.BroadCast(this, msg)
	}
}

// 给当前 User 对应的客户端发送消息
func (this *User) SendMsg(msg string) {
	this.conn.Write([]byte(msg))
}

// 读主 routine 信息，发送给客户端
func (this *User) ListenMessage() {
	for {
		msg := <-this.C

		this.conn.Write([]byte(msg + "\n"))
	}
}
