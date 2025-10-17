package main

import (
	"log"
	proxy "sqlserver_proxy/internal/proxy"
	"time"
)

func main() {

	p, err := proxy.NewProxy()
	if err != nil {
		log.Fatal(err)
	}

	err = p.Start()
	if err != nil {
		log.Fatal(err)
	}

	p.ShutdownWait(5 * time.Second)
}
