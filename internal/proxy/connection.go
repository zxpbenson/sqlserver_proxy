package proxy

import (
	"context"
	"io"
	"log"
	"net"
	"time"
)

type Connection struct {
	id     uint64
	server net.Conn
	client net.Conn
	node   *Node
	ctx    context.Context
}

func NewConnection(ctx context.Context, id uint64, server net.Conn, client net.Conn, node *Node) *Connection {
	conn := &Connection{
		id:     id,
		server: server,
		client: client,
		node:   node,
		ctx:    ctx,
	}
	return conn
}

func (c *Connection) IOCopy() {
	go c.doCopy()
}

func (c *Connection) doCopy() {
	defer c.client.Close()
	defer c.server.Close()
	defer c.node.conns.Delete(c.id)

	s2cDone := make(chan error)
	c2sDone := make(chan error)

	// 双向拷贝
	go func() {
		_, err := io.Copy(c.client, c.server)
		s2cDone <- err
	}()
	go func() {
		_, err := io.Copy(c.server, c.client)
		c2sDone <- err
	}()

	c.copyWait(s2cDone, "server", "client")
	c.copyWait(c2sDone, "client", "server")
}

func (c *Connection) copyWait(doneChan chan error, src string, dest string) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	//pass := false
	for {
		select {
		case <-c.ctx.Done():
			_ = c.client.SetReadDeadline(time.Now())
			_ = c.server.SetReadDeadline(time.Now())
			return
		case err := <-doneChan:
			if err != nil {
				log.Printf("TCP IO copy from %s to %s error: %v\n", src, dest, err)
				//return
			}
			//pass = true
			//break // 在 Go 里，break 语句默认只退出它所在的最近的 for、switch 或 select 语句块。 这里break只能退出select
			return
		case <-ticker.C:
			// 定期延长两边的 deadline，保持活跃
			_ = c.client.SetDeadline(time.Now().Add(5 * time.Second))
			_ = c.server.SetDeadline(time.Now().Add(5 * time.Second))
		}
		//if pass {
		//	break
		//}
	}
}
