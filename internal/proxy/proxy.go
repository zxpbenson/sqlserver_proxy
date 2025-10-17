package proxy

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sqlserver_proxy/pkg/util"
	"syscall"
	"time"
)

type Proxy struct {
	nodes              []*Node       // 节点列表
	listenPort         int           // proxy service 监听的端口
	checkInterval      time.Duration // 检查节点可用性的间隔
	config             string        // 节点配置文件路径
	listenerCloserFlag chan bool     // 用于关闭监听器的标志位
	acceptorFlag       chan bool     // 用于关闭 acceptor 的标志位
	nodeDetectorFlag   chan bool     // 用于关闭节点检测器的标志位
	ctx                context.Context
}

func NewProxy() (*Proxy, error) {
	p := &Proxy{}
	p.parseFlags()
	err := p.loadConfig()
	if err != nil {
		return nil, err
	}
	//err = p.loadConfigClassic()
	//if err != nil {
	//	return nil, err
	//}
	p.init()
	return p, nil
}

func (p *Proxy) init() { //Proxy初始化

	// 初始化全局Context和CancelFunc
	globalCtx, globalCancel := context.WithCancel(context.Background())

	// 监听系统信号
	go func() {
		sigChan := make(chan os.Signal, 1)
		// 监听 SIGINT (Ctrl+C) 和 SIGTERM (kill)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// 阻塞直到接收到信号
		sig := <-sigChan
		log.Printf("Received signal: %s\n", sig)

		// 取消全局Context
		globalCancel()

		// 可选：等待一段时间后强制退出
		//time.Sleep(5 * time.Second)
		//os.Exit(1)
	}()

	p.ctx = globalCtx
	p.acceptorFlag = make(chan bool)
	p.listenerCloserFlag = make(chan bool)
	p.nodeDetectorFlag = make(chan bool)
	for _, node := range p.nodes {
		node.init(p)
	}
}

func (p *Proxy) parseFlags() {
	flag.StringVar(&p.config, "config", "nodes.json", "Nodes config in json format, default nodes.json")
	flag.DurationVar(&p.checkInterval, "interval", time.Second*10, "Interval for checking the status of database nodes, default 10s")
	flag.IntVar(&p.listenPort, "port", 1433, "Port to listen on for proxy service, default 1433")
	flag.Parse()
}

func (p *Proxy) loadConfig() error {
	// 打开 JSON 文件
	file, err := os.Open(p.config)
	if err != nil {
		return fmt.Errorf("open config file error: %w", err)
	}
	defer file.Close()
	// 读取文件内容并解码
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&p.nodes); err != nil {
		return fmt.Errorf("decode config error: %w", err)
	}

	// 打印读取后的内容
	//log.Printf("Loaded config: %+v\n", p.nodes)
	// 或者打印成格式化的 JSON 形式（更易读）
	pretty, _ := json.MarshalIndent(p.nodes, "", "  ")
	log.Printf("Loaded config (pretty): %s\n", string(pretty))

	return nil
}

func (p *Proxy) loadConfigClassic() error {
	// 读取整个文件内容
	data, err := os.ReadFile(p.config)
	if err != nil {
		return fmt.Errorf("read config file error: %w", err)
	}

	// 打印原始文件内容（即使 JSON 格式有误也能看到）
	log.Printf("Raw config file content: %s\n", string(data))

	// 解码 JSON
	if err := json.Unmarshal(data, &p.nodes); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	return nil
}

func (p *Proxy) selectNode() (*Node, error) {
	for _, node := range p.nodes {
		if node.Enabled {
			return node, nil
		}
	}
	return nil, fmt.Errorf("there is not any enabled node")
}

func (p *Proxy) startServ() error {
	proxyListen := fmt.Sprintf(":%d", p.listenPort)

	lc := net.ListenConfig{}
	listener, err := lc.Listen(p.ctx, "tcp", proxyListen) //listener 是可退出的监听器
	if err != nil {
		return err
	}
	log.Printf("Proxy listening on : %s\n", proxyListen)

	go func() {
		defer close(p.listenerCloserFlag)
		defer log.Println("Proxy server listener stopped")
		<-p.ctx.Done()
		log.Println("Proxy server listener got signal, shutting down")
		err := listener.Close()
		if err != nil {
			log.Printf("Proxy server listener close error: %v\n", err)
		}
	}()

	go func(listener net.Listener) {
		defer close(p.acceptorFlag)
		defer log.Println("Proxy server stop accepting connections")
		for {
			if util.IsContextDone(p.ctx) {
				log.Println("Proxy server acceptor got signal, shutting down")
				return
			}
			conn, err := listener.Accept() // Listener.Accept() 会由ctx触发退出
			if err != nil {
				log.Printf("Proxy server listener accept error: %v\n", err)
				continue
			}
			node, err := p.selectNode()
			if err != nil {
				log.Printf("Proxy node select error: %v\n", err)
				err = conn.Close()
				if err != nil {
					log.Printf("Client tcp conn close error: %v\n", err)
				}
				continue
			}
			log.Printf("Proxy server accept a new connection from %s\n", conn.RemoteAddr())
			node.handleConn(conn)
		}
	}(listener)

	return nil
}

func (p *Proxy) startDetectNode() {
	go func() {
		defer close(p.nodeDetectorFlag)
		defer log.Println("Proxy server node detector stopped")
		var err error
		for {
			if util.IsContextDone(p.ctx) {
				return
			}
			for _, node := range p.nodes {
				err = node.detect()
				if err != nil {
					log.Printf("detect node fail, node : %s:%d, error : %v\n", node.Host, node.Port, err)
				}
			}
			subCtx, cancel := context.WithTimeout(p.ctx, p.checkInterval)
			<-subCtx.Done()
			cancel()
		}
	}()
}

func (p *Proxy) Start() error {
	p.startDetectNode()
	return p.startServ()
}

func (p *Proxy) ShutdownWait(timeout time.Duration) {
	<-p.ctx.Done()

	select {
	case <-time.After(timeout):
		log.Println("Proxy server detect node shutdown timeout")
	case <-p.nodeDetectorFlag:
		log.Println("Shutting down proxy server node detector gracefully")
	}
	select {
	case <-time.After(timeout):
		log.Println("Proxy server acceptor shutdown timeout")
	case <-p.acceptorFlag:
		log.Println("Shutting down proxy server acceptor gracefully")
	}
	select {
	case <-time.After(timeout):
		log.Println("Proxy server detect node shutdown timeout")
	case <-p.listenerCloserFlag:
		log.Println("Shutting down proxy server listener gracefully")
	}
}
