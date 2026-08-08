package core

import (
	"fmt"
	"javinkv/config"
	"log"
	"os"
	"strings"
)

func dumpKey(fp *os.File, key string, obj *Obj) {
	cmd := fmt.Sprintf("SET %s %s", key, obj.Value)
	tokens := strings.Split(cmd, " ")
	fp.Write(Encode(tokens, false))
}

func DumpAllAOF() {
	fp, err := os.OpenFile(
		config.AOFFile,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)
	if err != nil {
		log.Println("open AOF:", err)
		return
	}
	defer fp.Close()

	log.Println("rewriting AOF File at", config.AOFFile)
	for k, obj := range store {
		dumpKey(fp, k, obj)
	}

	log.Println("AOF File rewrite complete")
}
