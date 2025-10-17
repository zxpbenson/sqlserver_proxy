package proxy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"

	mssql "github.com/denisenkom/go-mssqldb"
)

type Node struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Password     string `json:"password"`
	Database     string `json:"database"`
	Enabled      bool
	proxy        *Proxy
	dsConnStr    string
	sqlDetectStr string
	tcpDialStr   string
	conns        sync.Map
	seq          uint64
	ctx          context.Context
}

func (n *Node) genSeq() uint64 {
	n.seq++
	return n.seq
}

// 代理 TCP
func (n *Node) handleConn(client net.Conn) {
	log.Printf("Dial tcp address : %s", n.tcpDialStr)
	server, err := net.Dial("tcp", n.tcpDialStr)
	if err != nil {
		log.Printf("Connect server failed: %v\n", err)
		defer func() {
			err := client.Close() // 无法连接到Server端，关闭客户端连接
			if err != nil {
				log.Printf("Database client close error: %v\n", err)
			}
		}()
		return
	}
	n.seq++
	conn := NewConnection(n.ctx, n.seq, server, client, n)
	n.conns.Store(n.seq, conn)
	conn.IOCopy()
}

func (n *Node) init(p *Proxy) { // 初始化节点
	n.ctx = p.ctx
	n.seq = 0
	n.proxy = p
	n.dsConnStr = fmt.Sprintf(
		"server=%s;port=%d;user id=%s;password=%s;database=%s;encrypt=disable",
		n.Host,
		n.Port,
		n.User,
		n.Password,
		n.Database)
	n.sqlDetectStr = fmt.Sprintf("SELECT mirroring_role_desc FROM sys.database_mirroring WHERE database_id=DB_ID('%s')", n.Database)
	n.tcpDialStr = fmt.Sprintf("%s:%d", n.Host, n.Port)
}

func (n *Node) detect() error {
	db, err := sql.Open("sqlserver", n.dsConnStr)
	if err != nil {
		n.Enabled = false
		return fmt.Errorf("sql.Open err: %v", err)
	}
	defer func() {
		err := db.Close()
		if err != nil {
			log.Printf("Database server close error: %v\n", err)
		}
	}()

	var role string
	err = db.QueryRow(n.sqlDetectStr).Scan(&role)
	if err != nil {
		n.Enabled = false

		var mssqlErr mssql.Error
		if errors.As(err, &mssqlErr) {
			if mssqlErr.Number == 4063 { // 忽略此错误
				return nil
			} else {
				log.Printf("SQLServer error: \nno -> %d, \nmessage -> %s\n", mssqlErr.Number, mssqlErr.Message)
			}
		}
		return fmt.Errorf("query row err: %v", err)
	}

	if role == "PRINCIPAL" {
		if n.Enabled == false {
			log.Printf("Node %s:%d is a principal node\n", n.Host, n.Port)
		}
		n.Enabled = true
	} else {
		if n.Enabled == true {
			log.Printf("Node %s:%d is not a principal node\n", n.Host, n.Port)
		}
		n.Enabled = false
	}
	return nil
}
