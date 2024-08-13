package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	//	"bytes"

	"bytes"
	"math"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"

	"6.5840/labgob"
	"6.5840/labrpc"
)

const (
	ELECTION_TIMEOUT = 200
	HEARTBEAT        = 100 * time.Millisecond
)

type State string

const (
	FOLLOWER  = "FOLLOWER"
	CANDIDATE = "CANDIDATE"
	LEADER    = "LEADER"
)

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	// 2A
	currentTerm int     // raft节点当前的周期
	votedFor    int     // 在当前获得选票的候选⼈的Id
	logs        []Entry // 复制日志队列
	leaderId    int     // 当前领导人的Id
	state       State   // 本节点的角色

	electionTimer  *time.Ticker // 选举计时器
	heartbeatTimer *time.Ticker // 心跳包计时器
	// 2B
	commitIndex  int   // 已知的最大被提交的日志条目的索引值
	lastApplied  int   // 最后被应用到状态机的日志条目索引值
	nextIndex    []int // 对于每⼀个服务器，需要发送给他的下⼀个日志条目的索引值（初始化为领导⼈最后索引值加⼀）
	matchIndex   []int // 对于每⼀个服务器，已经复制给他的日志的最高索引值
	applyCh      chan ApplyMsg
	applyMsgCond *sync.Cond // 提交日志到上层应用的条件变量 只有在成功提交新的日志的时候才会触发
}

type Entry struct {
	Command interface{}
	Term    int
	Index   int
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (2A).
	rf.mu.Lock()
	term = rf.currentTerm
	isleader = rf.state == LEADER
	rf.mu.Unlock()

	return term, isleader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {

	// rf.mu.Lock()
	// defer rf.mu.Unlock()

	buf := new(bytes.Buffer)

	encoder := labgob.NewEncoder(buf)
	encoder.Encode(rf.votedFor)
	encoder.Encode(rf.currentTerm)
	encoder.Encode(rf.logs)

	raftState := buf.Bytes()

	rf.persister.Save(raftState, nil)

	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}

	buf := bytes.NewBuffer(data)
	decoder := labgob.NewDecoder(buf)
	var votedFor int

	var currentTerm int
	var logs []Entry

	if decoder.Decode(&votedFor) != nil || decoder.Decode(&currentTerm) != nil || decoder.Decode(&logs) != nil {
		panic("read Persist error, please retry again")
	}
	rf.logs = logs
	rf.currentTerm = currentTerm
	rf.votedFor = votedFor

	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).

}

// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int // 候选⼈的任期号
	CandidateId  int // 请求选票的候选⼈的 Id
	LastLogIndex int // 候选⼈的最后⽇志条⽬的索引值
	LastLogTerm  int // 候选⼈最后⽇志条⽬的任期号
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int  // 当前任期号，以便于候选⼈去更新自己的任期号
	VoteGranted bool // 候选⼈赢得了此张选票时为真
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).

	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist()
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm // 有更新的当前任期号
		reply.VoteGranted = false
		return
	}

	if args.Term > rf.currentTerm {
		if rf.state != FOLLOWER {
			rf.changeState(FOLLOWER)
		}

		rf.currentTerm, rf.votedFor = args.Term, -1
	}
	// 执行到这里说明 请求参数中的任期大于等于当前节点的任期

	// 2A
	// 如果候选人的任期大于当前任期 那么必定请求投票成功
	// 如果候选人的任期等于当前任期 并且 （当前节点没有给其他节点投过票 或者 已经给当前候选人投过票（RPC可能发生重传））
	if rf.votedFor == -1 || rf.votedFor == args.CandidateId {
		// 2B 加上日志限制 日志需要是最新的
		if rf.isLogMatch(args.LastLogIndex, args.LastLogTerm) {
			rf.changeState(FOLLOWER)       // 当前节点状态变成follower
			rf.votedFor = args.CandidateId // 给候选者投票
			reply.VoteGranted = true
		} else {
			reply.VoteGranted = false
		}

	} else {
		// 已经给其他候选人投过票了
		reply.VoteGranted = false
	}

	reply.Term = rf.currentTerm
}

// 论文原话：通过⽐较两份日志中最后⼀条日志条目的索引值和任期号定义谁的日志⽐较新。如果两份
// 日志最后的条⽬的任期号不同，那么任期号⼤的⽇志更加新。如果两份⽇志最后的条⽬任期号
// 相同，那么⽇志⽐较⻓的那个就更加新
func (rf *Raft) isLogMatch(lastLogIndex int, lastLogTerm int) bool { //比谁更新
	lastLog := rf.getLastLog()
	if lastLogTerm > lastLog.Term || (lastLogTerm == lastLog.Term && lastLogIndex >= lastLog.Index) {
		return true
	}

	return false
}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

// AppendEntriesArgs 附加日志和心跳包的RPC请求参数
type AppendEntriesArgs struct {
	Term         int     // 领导⼈的任期号
	LeaderId     int     // 领导⼈的Id，以便于跟随者重定向请求
	PreLogIndex  int     // 新的日志条目紧随之前的索引值
	PreLogTerm   int     // prevLogIndex 条目的任期号
	Logs         []Entry // 准备存储的日志条目
	LeaderCommit int     // 领导⼈已经提交的日志的索引值
}

// AppendEntriesReply 附加日志和心跳包的RPC回复参数
type AppendEntriesReply struct {
	Term    int  // 当前的任期号，⽤于领导⼈去更新⾃⼰
	Success bool // 跟随者包含了匹配上 prevLogIndex 和 prevLogTerm 的⽇志时为真

	ConfilctTerm  int
	ConflictIndex int
}

// 发送追加日志和心跳包的RPC请求
func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	DPrintf("node {%d} term {%d} receive new log length %d\n", rf.me, rf.currentTerm, len(args.Logs))
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist()
	// 任期过期 什么也不做， 也就是无效的请求即可了
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	// 2A 任期相同 或 本节点任期更小
	// 节点维持follower状态不变 并且更新任期
	rf.changeState(FOLLOWER)
	rf.currentTerm = args.Term
	reply.Term = rf.currentTerm

	if args.PreLogIndex < rf.getFirstLog().Index {
		reply.Term, reply.Success = 0, false
		return
	}

	DPrintf("node {%d} term {%d} start judge whether append log. rf.logs:%v,args.Logs:%v\n", rf.me, rf.currentTerm, rf.logs, args.Logs)
	// 如果上一条日志索引对应的日志 在本节点的日志队列中不存在 或者无法对应上 那么返回false 表示日志队列不匹配
	if args.PreLogIndex > rf.getLastLog().Index || rf.logs[args.PreLogIndex-rf.getFirstLog().Index].Term != args.PreLogTerm {
		lastIndex := rf.getLastLog().Index
		if lastIndex < args.PreLogIndex {
			reply.ConfilctTerm = -1
			reply.ConflictIndex = lastIndex + 1
		} else {
			firstIndex := rf.getFirstLog().Index
			reply.ConfilctTerm = rf.logs[args.PreLogIndex-firstIndex].Term
			index := args.PreLogIndex - 1
			for index >= firstIndex && rf.logs[index-firstIndex].Term == reply.ConfilctTerm {
				index--
			}
			reply.ConflictIndex = index
		}

		reply.Success = false
		return
	}

	DPrintf("node {%d} term {%d} start append log\n", rf.me, rf.currentTerm)

	firstIndex := rf.cropLogs(args)
	// 附加日志
	rf.logs = append(rf.logs, args.Logs[firstIndex:]...)
	DPrintf("node {%d} term {%d} append log success. rf.logs:%v\n", rf.me, rf.currentTerm, rf.logs)
	// 求出新的commitIndex
	newCommitIndex := int(math.Min(float64(args.LeaderCommit), float64(rf.getLastLog().Index)))
	// 通知有新日志提交 如果是重传了已经加入日志队列的数据 那么commitIndex是不会变的 因此加入一个判断
	if newCommitIndex > rf.commitIndex {
		DPrintf("node {%d} term {%d} update commitIndex %d to %d\n", rf.me, rf.currentTerm, rf.commitIndex, newCommitIndex)
		rf.commitIndex = newCommitIndex
		rf.applyMsgCond.Signal()
	}

	reply.Success = true
}

// 裁剪日志 并返回最后一个匹配的索引
func (rf *Raft) cropLogs(args *AppendEntriesArgs) int {
	// 得到第一条日志的索引
	firstIndex := rf.getFirstLog().Index

	// 遍历新加入的日志
	for i, entry := range args.Logs {
		// 一旦新加入日志条目的索引超出了日志队列的范围 那么原队列不需要裁剪 把接收到的日志中不存在的日志添加到本地日志队列
		// 如果某个新加入的日志对应的任期不等于本地日志对应的任期 那么本地日志从这个位置以及以后的日志全部丢弃
		if entry.Index >= firstIndex+len(rf.logs) || rf.logs[entry.Index-firstIndex].Term != entry.Term {
			var tmp []Entry
			rf.logs = append(tmp, rf.logs[:entry.Index-firstIndex]...)
			return i
		}
	}

	return len(args.Logs)
}

func (rf *Raft) appendLog(command interface{}) Entry {
	entry := Entry{
		Term:    rf.currentTerm,
		Command: command,
		Index:   rf.getLastLog().Index + 1,
	}

	rf.logs = append(rf.logs, entry)
	rf.persist()
	return entry
}

// 得到日志队列的第一条日志 这个函数是为了 后续的快照做准备 如果使用快照的话 日志的索引就不等于日志在日志队列中的索引了
func (rf *Raft) getFirstLog() Entry {
	return rf.logs[0]
}

// 得到日志队列的最后一条日志
func (rf *Raft) getLastLog() Entry {
	return rf.logs[len(rf.logs)-1]
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).

	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state != LEADER {
		return -1, -1, false
	}

	DPrintf("leader node {%d} term {%d} receive command %v\n", rf.me, rf.currentTerm, command)
	newLog := rf.appendLog(command) // 日志队列中加入一条新log
	index = newLog.Index
	term = rf.currentTerm         // 当前的任期
	isLeader = rf.state == LEADER // 是否是leader

	return index, term, isLeader
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) ticker() {
	for rf.killed() == false {

		// Your code here (2A)
		// Check if a leader election should be started.

		select {
		case <-rf.electionTimer.C:
			DPrintf("node {%d} term {%d} start election\n", rf.me, rf.currentTerm)
			rf.mu.Lock()
			// FOLLOWER --> CANDIDATE
			rf.changeState(CANDIDATE)
			// 重置选举超时计时器
			rf.resetElectionTimeout()
			rf.requestVotes()
			rf.mu.Unlock()

		case <-rf.heartbeatTimer.C:
			// 只有领导人可以发送心跳包
			rf.mu.Lock()
			if rf.state == LEADER {
				DPrintf("node {%d} term {%d} send heartbeat\n", rf.me, rf.currentTerm)

				rf.sendEntries()

			}
			rf.mu.Unlock()
		}

	}
}

func (rf *Raft) applyMsg() {
	for rf.killed() == false {

		rf.mu.Lock()
		for rf.lastApplied >= rf.commitIndex {
			rf.applyMsgCond.Wait()
		}
		applyEntries := make([]Entry, rf.commitIndex-rf.lastApplied)
		firstIndex := rf.getFirstLog().Index
		commitIndex := rf.commitIndex
		copy(applyEntries, rf.logs[rf.lastApplied-firstIndex+1:rf.commitIndex-firstIndex+1]) //从现存有数据的位置的后一个开始
		rf.mu.Unlock()

		DPrintf("node {%d} term {%d} start commit log %v\n", rf.me, rf.currentTerm, applyEntries)
		for _, entry := range applyEntries {
			rf.applyCh <- ApplyMsg{
				CommandValid: true,
				Command:      entry.Command,
				CommandIndex: entry.Index,
			}
		}

		rf.mu.Lock()
		if rf.commitIndex > rf.lastApplied {
			rf.lastApplied = commitIndex
		}
		rf.mu.Unlock()

	}
}

func (rf *Raft) changeState(state State) {
	if state == FOLLOWER {
		DPrintf("node {%d} term {%d} change ==> follower\n", rf.me, rf.currentTerm)
		rf.resetElectionTimeout()
		rf.heartbeatTimer.Stop()
	} else if state == LEADER {
		DPrintf("node {%d} term {%d} change ==> leader\n", rf.me, rf.currentTerm)
		lastLog := rf.getLastLog()
		for i := 0; i < len(rf.peers); i++ {
			rf.matchIndex[i], rf.nextIndex[i] = 0, lastLog.Index+1
		}
		rf.electionTimer.Stop()
		rf.heartbeatTimer.Reset(HEARTBEAT)
	}

	rf.state = state
}

// 候选者向其他节点请求投票
func (rf *Raft) requestVotes() {
	totalVotes := len(rf.peers)

	rf.currentTerm += 1 // 任期加一
	rf.votedFor = rf.me // 自己给自己投票
	numVotes := 1       // 自己给自己投票
	rf.persist()
	for i := range rf.peers {
		if rf.me != i {

			go func(peer int) {
				// 发送RPC请求
				rf.mu.Lock()
				args := &RequestVoteArgs{
					Term:         rf.currentTerm,
					CandidateId:  rf.me,
					LastLogIndex: rf.getLastLog().Index, // 最后一条日志索引
					LastLogTerm:  rf.getLastLog().Term,  // 最后一条日志任期
				}
				rf.mu.Unlock()

				reply := &RequestVoteReply{}

				// 需要成功接收再执行
				if rf.sendRequestVote(peer, args, reply) {

					rf.mu.Lock()
					defer rf.mu.Unlock()

					// 如果不加入这个判断 那么当节点较多的时候会执行多次转换为leader或follower节点的操作
					// 如果其他节点任期更大 当前节点会转换为follower 同时任期更新到最新 后面的投票请求不应该再发送 所以要加个args.Term == rf.currentTerm的判断
					if rf.state == CANDIDATE && rf.currentTerm == args.Term {
						// 请求投票成功 那么票数+1
						if reply.VoteGranted {
							numVotes += 1

							if numVotes > totalVotes/2 {
								// 本节点当选Leader 选举停止 开始发送心跳包
								rf.changeState(LEADER)
								// 选举成功需要马上把心跳包发送给其他节点 则可能出现leader已经存在 其他节点还在选举的情况
								rf.sendEntries()
							}
						}

						// 已经出现了任期更大的节点 本节点强制转换为FOLLOWER
						if reply.Term > rf.currentTerm {
							rf.currentTerm = reply.Term
							rf.votedFor = -1
							// 要转换成跟随者，这个才是依据原文的
							rf.changeState(FOLLOWER)
							rf.persist()
						}
					}

				}
			}(i)
		}
	}

}

func (rf *Raft) sendEntries() {
	for i := range rf.peers {
		if rf.me != i {
			go func(peer int) {
				rf.mu.Lock()
				firstIndex := rf.getFirstLog().Index
				// 发送RPC请求
				args := &AppendEntriesArgs{
					Term:         rf.currentTerm,
					LeaderId:     rf.me,
					PreLogIndex:  rf.nextIndex[peer] - 1,                        // 发送日志条目上一条日志的索引
					PreLogTerm:   rf.logs[rf.nextIndex[peer]-1-firstIndex].Term, // 发送日志条目上一条日志的任期 匹配才接收同步日志
					Logs:         rf.logs[rf.nextIndex[peer]-firstIndex:],       // 下一批要同步过去的日志
					LeaderCommit: rf.commitIndex,                                // leader已经提交的日志
				}
				rf.mu.Unlock()

				reply := &AppendEntriesReply{}
				if rf.sendAppendEntries(peer, args, reply) {
					rf.mu.Lock()
					// 2A

					if rf.state == LEADER && rf.currentTerm == args.Term {
						if reply.Term > rf.currentTerm {
							rf.currentTerm = reply.Term
							rf.votedFor = -1
							rf.changeState(FOLLOWER)
							rf.persist()
						} else {
							// 2B 对方任期等于当前任期 或小于当前任期 那么判断日志接收的情况
							if reply.Success {

								rf.matchIndex[peer] = args.PreLogIndex + len(args.Logs)

								rf.nextIndex[peer] = rf.matchIndex[peer] + 1

								rf.updateLeaderCommit()
							} else {
								// 自检 从上一个索引开始判断是否匹配
								// rf.nextIndex[peer] -= 1
								rf.nextIndex[peer] = reply.ConflictIndex
								if reply.ConfilctTerm != -1 {
									firstIndex := rf.getFirstLog().Index
									for i := args.PreLogIndex; i >= firstIndex; i-- {
										if rf.logs[i-firstIndex].Term == reply.ConfilctTerm {
											rf.nextIndex[peer] = i + 1
											break
										}
									}
								}
							}
						}

					}
					rf.mu.Unlock()

				}
			}(i)
		}
	}
}

func (rf *Raft) updateLeaderCommit() {
	// 对matchIndex数组排序 然后求其中位数即可，中位数过了就算能commit了
	l := len(rf.matchIndex)
	sortMatchIndex := make([]int, l)

	// sortMatchIndex := make([]int, len(rf.matchIndex))
	copy(sortMatchIndex, rf.matchIndex)
	sortMatchIndex[rf.me] = rf.getLastLog().Index
	sort.Ints(sortMatchIndex)

	newCommitIndex := sortMatchIndex[l/2]
	if newCommitIndex > rf.commitIndex && newCommitIndex <= rf.getLastLog().Index && rf.logs[newCommitIndex-rf.getFirstLog().Index].Term == rf.currentTerm { //不是同一个任期那就不是我负责了
		DPrintf("leader node {%d} term {%d} update commitIndex %d to %d\n", rf.me, rf.currentTerm, rf.commitIndex, newCommitIndex)
		rf.commitIndex = newCommitIndex
		rf.applyMsgCond.Signal()
	}
}

// 重置本节点的超时截止时间
func (rf *Raft) resetElectionTimeout() {
	rf.electionTimer.Reset(randomElectionTime())
}

func randomElectionTime() time.Duration {

	ms := rand.Int63() % 100

	eleTime := time.Duration(ELECTION_TIMEOUT+ms) * time.Millisecond
	return eleTime
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	// 2A
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.state = FOLLOWER

	// 初始化的时候开启选举计时器 关闭心跳包 只有当选了leader才开启心跳包
	rf.electionTimer = time.NewTicker(randomElectionTime())
	rf.heartbeatTimer = time.NewTicker(HEARTBEAT)
	rf.heartbeatTimer.Stop()
	// 2B
	rf.logs = make([]Entry, 1) // 设置第一个日志条目为空日志
	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))
	rf.applyMsgCond = sync.NewCond(&rf.mu)
	rf.applyCh = applyCh
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	lastLog := rf.getLastLog()
	for i := 0; i < len(rf.peers); i++ {
		rf.nextIndex[i] = lastLog.Index + 1
	}

	// start ticker goroutine to start elections
	go rf.ticker()
	go rf.applyMsg()
	return rf
}
