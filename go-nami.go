package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"

	"github.com/nstehr/go-nami/client"
	"github.com/nstehr/go-nami/encoder"
	"github.com/nstehr/go-nami/server"
	"github.com/nstehr/go-nami/shared/transfer"
)

func main() {
	e := encoder.BsonEncoder{}
	t := flag.String("type", "server", "")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")
	filename := flag.String("file", "", "filename")
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			log.Println(sig)
			pprof.StopCPUProfile()
			os.Exit(1)
			return
		}
	}()
	if *t == "client" {
		config := transfer.NewConfig()
		c := client.NewClient("Downloads", config, e)
		statusCh := c.GetFile(*filename, ":46224")
		for status := range statusCh {
			log.Println(status)
		}

	} else {
		s := server.NewServer(e, 46224, "Uploads")
		s.StartListening()
	}

}
