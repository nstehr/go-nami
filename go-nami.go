package main

import (
	"flag"
	"log"

	"github.com/nstehr/go-nami/client"
	"github.com/nstehr/go-nami/encoder"
	"github.com/nstehr/go-nami/server"
)

func main() {
	e := encoder.GobEncoder{}
	t := flag.String("type", "server", "")
	flag.Parse()
	if *t == "client" {
		c := client.NewClient(e)
		statusCh := c.GetFile("foo.txt", ":46224")
		for status := range statusCh {
			log.Println(status)
		}

	} else {
		s := server.NewServer(e, 46224)
		s.StartListening()
	}

}
