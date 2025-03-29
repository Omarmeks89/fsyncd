package main

import (
	"log"
)

func main() {
	sm := MakeSyncMeta()
	if err := sm.MakeMeta("a"); err != nil {
		log.Fatal(err)
	}
}
