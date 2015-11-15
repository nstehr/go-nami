package main

import (
	"flag"
	"log"

	"github.com/nstehr/go-nami/client"
	"github.com/nstehr/go-nami/encoder"
	"github.com/nstehr/go-nami/server"
	"github.com/nstehr/go-nami/shared/transfer"
)

func main() {
	e := encoder.NewGobEncoder()
	t := flag.String("type", "server", "")
	flag.Parse()
	if *t == "client" {
		config := transfer.NewConfig()
		c := client.NewClient("Downloads", config, e)
		statusCh := c.GetFile("test.m4a", ":46224")
		for status := range statusCh {
			log.Println(status)
		}

	} else {
		s := server.NewServer(e, 46224, "Uploads")
		s.StartListening()
	}

}
