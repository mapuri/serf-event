package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/mapuri/serf/client"
	"github.com/mapuri/serfer"
)

func main() {
	r := serfer.NewRouter()
	r.AddMemberJoinHandler(handleJoin)

	if err := r.InitSerfAndServe(""); err != nil {
		log.Fatalf("Failed to initialize serfer. Error: %s", err)
	}
}

func handleJoin(name string, e client.EventRecord) {
	log.Infof("Received member join: %q: %v", name, e)
}
