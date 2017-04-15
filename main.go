package main

import (
	"fmt"
	"evac/server"
	"evac/caching"
)

/* TODO: 1 - Read incoming DNS request */
/* TODO: 2 - Check cache for request response */
/* TODO: 3 - Check blacklist for request domain */
/* TODO: 4 - Request from remote DNS server */
/* TODO: 5 - Serve DNS response to client */

func main() {
	caching.NewCache(200)
	listener := server.NewServer(50)
	go func () {
		err := listener.Start(":53")
		if err != nil {
			fmt.Print(err)
		}
	}()

	<-listener.IncomingRequests
	fmt.Print("Done")
}
