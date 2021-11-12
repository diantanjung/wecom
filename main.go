package main

import (
	"github.com/diantanjung/wecom/api"
	"github.com/diantanjung/wecom/db"
	db2 "github.com/diantanjung/wecom/db/sqlc"
	"github.com/diantanjung/wecom/util"
	"log"
)

const (
	//pathDir = "/Users/diantanjung/Projects/web/go/wecom/" //local
	pathDir = "/home/dian/go/bin/wecom2/" //production
)

func main() {
	config, err := util.LoadConfig(pathDir)
	if err != nil {
		log.Fatal("cannot load config:", err)
	}

	conn, err := db.Open(config)
	if err != nil {
		log.Fatal("cannot connect to db:", err)
	}

	store := db2.New(conn)

	server, err := api.NewServer(config, store)
	if err != nil {
		log.Fatal("cannot create server:", err)
	}

	server.Start(":9000")
}
