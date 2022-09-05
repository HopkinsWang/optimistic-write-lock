// Copyright 2017,2018 Lei Ni (nilei81@gmail.com).
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
linearizable is an example program for building a linearizable state machine using dragonboat.
*/
package main

import (
	"flag"
	"fmt"
	"github.com/lni/dragonboat/logger"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
)

const (
	ShardID uint64 = 128
)
var(
	localhostCmd = "ifconfig eth0 | grep \"inet \" | awk -F \":\" '{print $1}' | awk '{print $2}'"
)

//var (
//	datadir = "/tmp/dragonboat-example-linearizable"
//	members = map[uint64]string{
//		1: "localhost:61001",
//		2: "localhost:61002",
//		3: "localhost:61003",
//	}
//	httpAddr = []string{
//		":8001",
//		":8002",
//		":8003",
//	}
//	shardID uint64 = 128
//)

func main() {
	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt)
	signal.Notify(stop, syscall.SIGTERM)

	replicaID := flag.Int("replicaid", 1, "ReplicaID to use")
	addr := flag.String("addr", "", "Nodehost address")
	join := flag.Bool("join", false, "Joining a new node")
	bootstrap := flag.Bool("bootstrap", false, "a initialnode of cluster")
	flag.Parse()

	if runtime.GOOS == "darwin" {
		signal.Ignore(syscall.Signal(0xd))
	}
	initialMembers := make(map[uint64]string)
	if *addr =="" {
		panic("addr is null")
	}

	 exe_res, exe_err,_err:= ExecExternalScript(localhostCmd)
	 if _err!=nil || exe_err!=""{
		 log.Printf("exec_err: %v",_err)
	 }

	 old_addr := strings.Split(*addr,":")
	 newloaclAddr := fmt.Sprintf("%v:%v",exe_res,old_addr[1])

	 log.Printf("oldAddr: %v:%v, newAddr: %v",old_addr[0],old_addr[1],newloaclAddr)
	if *bootstrap{
		initialMembers[1] = newloaclAddr
	}

	logger.GetLogger("raft").SetLevel(logger.ERROR)
	logger.GetLogger("rsm").SetLevel(logger.WARNING)
	logger.GetLogger("transport").SetLevel(logger.WARNING)
	logger.GetLogger("grpc").SetLevel(logger.WARNING)

	datadir :=filepath.Join(
		"raft-cluster",
		"raft-data",
		fmt.Sprintf("node%d", *replicaID))

	nh, nnh_err := dragonboat.NewNodeHost(config.NodeHostConfig{
		RaftAddress:    newloaclAddr,
		NodeHostDir:    datadir,
		RTTMillisecond: 100,
	})
	if nnh_err!=nil{
		log.Printf("dragonboat.NewNodeHost Err!")
		panic(nnh_err)
	}

	fsm := NewLinearizableFSM()

	scr_err := nh.StartConcurrentReplica(initialMembers, *join, fsm, config.Config{
		ReplicaID:          uint64(*replicaID),
		ShardID:            ShardID,
		ElectionRTT:        10,
		HeartbeatRTT:       1,
		CheckQuorum:        true,
		SnapshotEntries:    10,
		CompactionOverhead: 5,
	})

	if scr_err!=nil{
		log.Printf("nh.StartConcurrentReplica!")
		panic(scr_err)
	}

	go func(s *http.Server) {
		log.Fatal(s.ListenAndServe())
	}(&http.Server{
		Addr:    newloaclAddr,
		Handler: &handler{nh},
	})
	<-stop
}
