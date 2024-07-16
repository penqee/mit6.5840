package kvsrv

import (
	"log"
	"sync"
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type ClientId int64

type KVServer struct {
	mu sync.Mutex

	kvMap map[string]string

	history sync.Map

	// Your definitions here.
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
	kv.mu.Lock()
	defer kv.mu.Unlock()

	reply.Value = kv.kvMap[args.Key]

}

func (kv *KVServer) Put(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.

	if args.Tag == REQUESTAGAIN {
		kv.history.Delete(args.Id)
		return
	}

	if value, ok := kv.history.Load(args.Id); ok {
		reply.Value = value.(string)

		return
	}

	kv.mu.Lock()
	defer kv.mu.Unlock()

	reply.Value = kv.kvMap[args.Key]
	kv.history.Store(args.Id, kv.kvMap[args.Key])
	kv.kvMap[args.Key] = args.Value

}

func (kv *KVServer) Append(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	if args.Tag == REQUESTAGAIN {
		kv.history.Delete(args.Id)
		return
	}
	if value, ok := kv.history.Load(args.Id); ok {
		reply.Value = value.(string)
		return
	}
	kv.mu.Lock()
	defer kv.mu.Unlock()

	reply.Value = kv.kvMap[args.Key]
	kv.history.Store(args.Id, kv.kvMap[args.Key])
	kv.kvMap[args.Key] += args.Value

}

func StartKVServer() *KVServer {
	kv := new(KVServer)

	kv = &KVServer{
		mu: sync.Mutex{},

		kvMap: make(map[string]string),

		history: sync.Map{},
	}
	// You may need initialization code here.

	return kv
}
