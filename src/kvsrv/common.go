package kvsrv

const (
	REQUEST      = "REQUEST"
	REQUESTAGAIN = "REQUESTAGAIN"
)

// Put or Append
type PutAppendArgs struct {
	Key   string
	Value string

	Tag string
	Id  int64
	// You'll have to add definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
}

type PutAppendReply struct {
	Value string
}

type GetArgs struct {
	Key string

	ReqId int64
	// You'll have to add definitions here.
}

type GetReply struct {
	Value string
}

type RegisterArgs struct {
}

type RegisterReply struct {
	Id ClientId
}
