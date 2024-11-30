package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
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

	"math"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labrpc"
)

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 3D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 3D:
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

	// Your data here (3A, 3B, 3C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	currentTerm int
	votedFor    int
	log         []Entry

	commitIndex int
	lastApplied int

	nextIndex  []int
	matchIndex []int

	electionTime      *time.Timer
	heartBeatTime     *time.Timer
	heartBeatInterval time.Duration

	identity  string
	voteCount int

	applyCh   chan ApplyMsg
	applyCond sync.Cond
}

const (
	FOLLOWER  = "FOLLOWER"
	CANDIDATE = "CANDIDATE"
	LEADER    = "LEADER"
)

type Entry struct {
	Command interface{}
	Index   int
	Term    int
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	rf.mu.Lock()
	term = rf.currentTerm
	isleader = rf.identity == LEADER
	rf.mu.Unlock()
	rf.mu.Lock()
	term = rf.currentTerm
	isleader = rf.identity == LEADER
	rf.mu.Unlock()
	// Your code here (3A).
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
	// Your code here (3C).
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
	// Your code here (3C).
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
	// Your code here (3D).

}

// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (3A, 3B).

	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (3A).

	Term        int
	VoteGranted bool
}

func (rf *Raft) GetFirstlog() Entry {
	return rf.log[0]
}

func (rf *Raft) GetLastLog() Entry {
	return rf.log[len(rf.log)-1]
}

func (rf *Raft) ValidateLog(term, index int) bool {
	lastLog := rf.GetLastLog()

	if (lastLog.Term < term) || (lastLog.Term == term && lastLog.Index <= index) {
		return true
	}
	return false
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (3A, 3B).

	rf.mu.Lock()
	defer rf.mu.Unlock()

	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		DPrintf("%d %s  S%d  due to less term,failed to vote at T%d", time.Now().Unix()%10000, dLeader, rf.me, rf.currentTerm)
		return
	}

	if args.Term > rf.currentTerm {
		rf.ChangeIdentity(FOLLOWER)
		// rf.electionTime.Reset(rf.GetRandomElectionTime())
		// rf.heartBeatTime.Stop()
		rf.currentTerm = args.Term

	} //好像心跳检测使用放在leader那里搞的

	//任期比我大我要投票，或者是我不是candidate的话也要投票
	//如果候选者的任期小于接收者的当前任期，则回复 false。这确保了只有来自至少与接收者处于同一任期或更高任期的候选者才能被考虑。
	// 如果 votedFor 为空或等于 candidateId，且候选者的日志至少与接收者的日志一样新，则授予投票
	if rf.votedFor == args.CandidateId || rf.votedFor == -1 {
		if rf.ValidateLog(args.LastLogTerm, args.LastLogIndex) {
			if rf.identity != FOLLOWER { // 这里是等于还是不等于？ 不是FOLLOWER也就只能是Candidate，因为LEADER的votedfor是它自己
				rf.ChangeIdentity(FOLLOWER)
			}

			// rf.electionTime.Reset(rf.GetRandomElectionTime())
			rf.votedFor = args.CandidateId
			reply.VoteGranted = true
			reply.Term = rf.currentTerm
			DPrintf("%d %s  S%d  vote for %d at T%d", time.Now().Unix()%10000, dLeader, rf.me, args.CandidateId, rf.currentTerm)
			return
		} else {
			reply.Term = rf.currentTerm
			reply.VoteGranted = false
		}

	} else {

		reply.Term = rf.currentTerm
		reply.VoteGranted = false
	}

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

	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.identity != LEADER {
		return index, term, false
	}

	lastLog := rf.GetLastLog()

	log := Entry{
		Command: command,
		Index:   lastLog.Index + 1,
		Term:    rf.currentTerm,
	}

	rf.log = append(rf.log, log)
	DPrintf("%d %s  S%d  put log[index=%d], term = %d at T%d", time.Now().Unix()%10000, dLeader, rf.me, log.Index, log.Term, rf.currentTerm)
	// Your code here (3B).

	index = log.Index
	term = log.Term

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

// ticker use to detect election time
// ticker use to detect election time
func (rf *Raft) ticker() {
	for rf.killed() == false {
		time.Sleep(10 * time.Millisecond)
		select {
		case <-rf.electionTime.C:
			rf.mu.Lock()
			rf.ChangeIdentity(CANDIDATE)
			rf.electionTime.Reset(rf.GetRandomElectionTime())
			rf.heartBeatTime.Stop()
			rf.currentTerm++

			DPrintf("%d %s  S%d  election time out, len(logs) = %d,restart election at T%d", time.Now().Unix()%10000, dLeader, rf.me, len(rf.log), rf.currentTerm)
			rf.mu.Unlock()
			// 发起选举的条件：如果选举超时：开始新的选举
			go rf.startElection()

		case <-rf.heartBeatTime.C:
			rf.mu.Lock()
			if rf.identity == LEADER {

				DPrintf("%d %s S%d heartbeat time out, resend heart beat checking election at T%d", time.Now().Unix()%10000, dTimer, rf.me, rf.currentTerm)
				rf.mu.Unlock()

				go rf.HeartBeat()

			} else {
				rf.mu.Unlock()
			}

		}

		// time.Sleep(rf.heartBeatInterval)
		// Your code here (3A)
		// Check if a leader election should be started.

		// pause for a random amount of time between 50 and 350
		// milliseconds.

	}
}

func (rf *Raft) GetRandomElectionTime() time.Duration {
	ms := 50 + (rand.Int63() % 300) //ms := 50 + (rand.Int63() % 300)
	return time.Duration(ms) * time.Millisecond
}

func (rf *Raft) ChangeIdentity(identity string) {
	if identity == FOLLOWER {
		// rf.electionTime.Reset(rf.GetRandomElectionTime())
		// rf.heartBeatTime.Stop()
		rf.voteCount = 0
		rf.votedFor = -1
		rf.identity = FOLLOWER
		rf.heartBeatTime.Stop()
		rf.electionTime.Reset(rf.GetRandomElectionTime()) //11.29暂时注释

		//这里就不对term进行自增了，因为不一定是只加一
	} else if identity == LEADER {
		rf.electionTime.Stop()
		rf.heartBeatTime.Reset(rf.heartBeatInterval)
		rf.identity = LEADER
		rf.voteCount = 0
		rf.votedFor = -1

		for i := 0; i < len(rf.matchIndex); i++ {
			rf.nextIndex[i] = rf.GetLastLog().Index + 1
			rf.matchIndex[i] = 0
		}

	} else {

		rf.votedFor = rf.me
		rf.voteCount = 1
		rf.identity = CANDIDATE
		// rf.electionTime.Reset(rf.GetRandomElectionTime())
	}
}

func (rf *Raft) startElection() {

	for i := 0; i < len(rf.peers); i++ {

		if i != rf.me {
			rf.mu.Lock()
			if rf.identity != CANDIDATE { // 不是候选人就失去了投票的意义

				rf.mu.Unlock()
				return
			}

			DPrintf("%d %s  S%d  send vote request to Server%d at T%d", time.Now().Unix()%10000, dVote, rf.me, i, rf.currentTerm)
			rf.mu.Unlock()
			go rf.voteRequest(i)
		}
	}
}

// voteRequest 负责投票逻辑
func (rf *Raft) voteRequest(server int) {
	rf.mu.Lock()

	if rf.identity == LEADER { //被别的线程修改了身份，直接退出
		DPrintf("%d %s S%d is not candidate,not need to vote req", time.Now().Unix()%10000, dLeader, rf.me)
		rf.mu.Unlock()
		return
	}

	args := &RequestVoteArgs{
		Term:         rf.currentTerm,
		CandidateId:  rf.me,
		LastLogIndex: rf.GetLastLog().Index,
		LastLogTerm:  rf.GetLastLog().Term,
	}
	DPrintf("%d %s S%d vote request value = %v at T%d", time.Now().Unix()%10000, dLeader, rf.me, args, rf.currentTerm)

	rf.mu.Unlock()
	reply := &RequestVoteReply{}

	if rf.sendRequestVote(server, args, reply) { // 选举有问题，日志过旧的不可能选举成功

		rf.mu.Lock()
		defer rf.mu.Unlock()
		if rf.identity != CANDIDATE || rf.currentTerm > reply.Term {
			DPrintf("%d %s S%d is not candidate,not need to vote req", time.Now().Unix()%10000, dLeader, rf.me)

			return
		}

		if reply.Term > rf.currentTerm {
			DPrintf("%d %s S%d term is old, reply term is %d,Convert to FOLLOWER at T%d", time.Now().Unix()%10000, dTimer, rf.me, reply.Term, rf.currentTerm)
			rf.ChangeIdentity(FOLLOWER) // 这里似乎不用重置选举计时器
			// rf.electionTime.Reset(rf.GetRandomElectionTime())
			// rf.voteCount = 0
			// rf.votedFor = -1
			rf.currentTerm = reply.Term
			return
		}
		if reply.VoteGranted {
			rf.voteCount++
			DPrintf("%d %s  S%d  receive vote from Server%d at T%d", time.Now().Unix()%10000, dVote, rf.me, server, rf.currentTerm)

			if rf.voteCount > len(rf.peers)/2 {
				DPrintf("%d %s  S%d  has %d votes,Convert to LEADER at T%d", time.Now().Unix()%10000, dLeader, rf.me, rf.voteCount, rf.currentTerm)
				rf.ChangeIdentity(LEADER)

				go rf.HeartBeat()
			}
		}

	}

}

func (rf *Raft) sendHeatBeat(server int) {

	rf.mu.Lock() //可能是锁滥用了导致心跳检测次数变少

	args := &AppendEntriesArgs{
		Term:     rf.currentTerm,
		LeaderId: rf.me,

		PrevLogTerm:  rf.log[rf.nextIndex[server]-rf.GetFirstlog().Index-1].Term,
		PrevLogIndex: rf.nextIndex[server] - 1, // 结构体的那个索引传进去是用不上的，应该是传这个
		LeaderCommit: rf.commitIndex,
		Entries:      rf.log[rf.nextIndex[server]-rf.GetFirstlog().Index:],
	}
	rf.mu.Unlock()
	reply := &AppendEntriesReply{}

	// if rf.identity != LEADER {
	// 	rf.mu.Unlock()
	// 	return
	// }
	// rf.mu.Unlock()
	if server != rf.me {
		if rf.sendAppendEntries(server, args, reply) {
			rf.mu.Lock()
			if rf.identity == LEADER && rf.currentTerm == args.Term {
				DPrintf("%d %s  S%d  send hearbeat checking to Server%d at T%d", time.Now().Unix()%10000, dLeader, rf.me, server, rf.currentTerm)
				// rf.mu.Lock()
				if reply.Term > rf.currentTerm {
					rf.currentTerm = reply.Term
					rf.ChangeIdentity(FOLLOWER)

				} else { // 任期大于等于FOLLOWE，那就开始准备改日志了
					if reply.Success { //日志完全匹配了
						DPrintf("%d %s  S%d  successfully hearbeat checking, PrevLogIndex = %d, len(args.Entries) = %d at T%d", time.Now().Unix()%10000, dLeader, rf.me, args.PrevLogIndex, len(args.Entries), rf.currentTerm)

						rf.matchIndex[server] = args.PrevLogIndex + len(args.Entries) //不能！因为nextIndex在此次RPC途中可能就被更新了，
						// 为了保证时时刻刻都是正确的，最好用prevLogIndex + len(send_entries)对应这次RPC reply成功时潜在的matchIndex更新(和自身比较取个Max)。
						rf.nextIndex[server] = rf.matchIndex[server] + 1
						rf.updateCommit() //问题出在args.Entries上，

					} else {

						for i := len(rf.log) - 1; i >= 0; i-- {
							if rf.log[i].Term == reply.ConflictTerm {
								rf.nextIndex[server] = rf.log[i].Index + 1
								break
							} else {
								rf.nextIndex[server] = reply.ConflictIndex
							}
						}

						// if reply.ConflictIndex != args.PrevLogIndex {
						// 	rf.nextIndex[server] = reply.ConflictIndex + 1
						// } else {

						// 	rf.nextIndex[server]--
						// }
						// rf.nextIndex[server]--
						// 日志没匹配好，需要修改nextIndex
						//  这个太慢了，用赋值快进
					}

				}

			}
			rf.mu.Unlock()
		}
	}
}

func (rf *Raft) HeartBeat() {

	// for !rf.killed() {
	// 	time.Sleep(rf.heartBeatInterval)

	for i := 0; i < len(rf.peers); i++ {
		rf.mu.Lock()
		if rf.identity != LEADER {
			return
		}
		rf.mu.Unlock()
		if i != rf.me {
			go rf.sendHeatBeat(i)
		}

	}
}

func (rf *Raft) updateCommit() { // 更新主leader的commit
	// 过半数提交了就leader提交
	commitList := make([]int, len(rf.matchIndex)) //matchindex比较稳定不容易变，就用这个来维护
	copy(commitList, rf.matchIndex)
	sort.Ints(commitList) // 进行排序，中位数过半即可是做提交

	mid := commitList[len(rf.matchIndex)/2+1]
	DPrintf("%d %s  S%d  commit situation: %v  %v  %v at T%d", time.Now().Unix()%10000, dLeader, rf.me, mid >= rf.commitIndex, mid <= rf.GetLastLog().Index, rf.log[mid-rf.GetFirstlog().Index].Term == rf.currentTerm, rf.currentTerm)
	DPrintf("%d %s  S%d  mid = %d, logTerm= %d at T%d", time.Now().Unix()%10000, dLeader, rf.me, mid, rf.log[mid-rf.GetFirstlog().Index].Term, rf.currentTerm)

	// 提交的条件：法定多数提交，log任期和当期任期一致，保证我的任期只管我任期的日志,  --- 这句话有点问题，如果自己以前的日志没提交的话，那岂不是直接没了？
	if mid >= rf.commitIndex && mid <= rf.GetLastLog().Index && rf.log[mid-rf.GetFirstlog().Index].Term <= rf.currentTerm {

		rf.commitIndex = mid
		rf.applyCond.Signal()
	}

}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	return rf.peers[server].Call("Raft.AppendEntries", args, reply)
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.currentTerm > args.Term {
		reply.Success = false
		reply.Term = rf.currentTerm
		return
	}

	rf.ChangeIdentity(FOLLOWER)

	rf.currentTerm = args.Term
	reply.Term = rf.currentTerm

	// 如果跟随者的日志中不包含与 prevLogIndex 和 prevLogTerm 匹配的日志条目，返回 false
	if args.PrevLogIndex < rf.GetFirstlog().Index { //自己的日志太短，和leader的日志差太多，就要立刻返回，不然没法处理
		reply.Term = 0
		reply.Success = false

		reply.ConflictIndex = -1
		reply.ConflictTerm = -1
		return
	}

	if args.PrevLogIndex > rf.GetLastLog().Index {
		reply.Success = false
		reply.ConflictIndex = rf.GetLastLog().Index + 1
		reply.ConflictTerm = -1
		return
	}

	// if args.PrevLogIndex > rf.GetLastLog().Index {
	// 	reply.Success = false
	// 	reply.ConflictIndex = rf.GetLastLog().Index
	// 	reply.ConflictTerm = rf.GetLastLog().Term
	// 	return
	// }

	// 如果跟随者的日志中不包含与 prevLogIndex 和 prevLogTerm 匹配的日志条目，返回 false
	// 如果已存在的日志条目与新日志条目冲突（索引相同但任期不同），删除已存在的条目及其之后所有条目。这能确保从领导者复制的日志条目的准确性。
	// if args.PrevLogIndex > rf.GetLastLog().Index || rf.log[args.PrevLogIndex-rf.GetFirstlog().Index].Term != args.PrevLogTerm {
	// 	reply.Success = false
	// 	return
	// }

	if rf.log[args.PrevLogIndex-rf.GetFirstlog().Index].Term != args.PrevLogTerm {
		reply.Success = false
		reply.ConflictTerm = rf.log[args.PrevLogIndex].Term //以leader的任期和索引为准
		for _, v := range rf.log {
			if v.Term != reply.ConflictTerm {
				continue
			}
			reply.ConflictIndex = v.Index
			break
		}
		return
	}

	// if rf.log[args.PrevLogIndex-rf.GetFirstlog().Index].Term != args.PrevLogTerm {
	// 	reply.Success = false
	// 	reply.ConflictIndex = args.PrevLogIndex
	// 	reply.ConflictTerm = args.PrevLogTerm //以leader的任期和索引为准
	// 	return
	// }

	// 开始比较log情况了
	firstArgsLogIndex := rf.ModifyLogs(args.Entries)
	rf.log = append(rf.log, args.Entries[firstArgsLogIndex:]...)

	// 如果领导者的已提交索引大于跟随者的已提交索引
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = int(math.Min(float64(args.LeaderCommit), float64(rf.GetLastLog().Index))) // 更新提交，开始apply

		//rf.applyCond.Signal()   !!! 这里存疑，foloower貌似不需要提交吧，复制就好了
	}
	// newCommit := int(math.Min(float64(args.LeaderCommit), float64(rf.GetLastLog().Index)))
	// if newCommit > rf.commitIndex {
	// 	rf.commitIndex = newCommit
	// 	rf.applyCond.Signal()
	// }
	//如果领导者的已提交索引大于跟随者的已提交索引，将跟随者的 commitIndex 设置为 leaderCommit 和最后一个新条目索引中的较小者。

	reply.Success = true

}

// 一种情况是logs的第一个日志索引比rf的最后一个日志大，应该返回rf的最后一个日志。一种情况是有部分匹配，那就挑选最后一个匹配的日志。
// 还有一种情况是rf的最大索引比logs的最大索引大，那就需要裁剪rf多出来的部分了
func (rf *Raft) ModifyLogs(logs []Entry) int { // 返回最后面的且能匹配的log

	firstLog := rf.GetFirstlog()
	lastLog := rf.GetLastLog()
	for i, log := range logs {
		// DPrintf("%d %s  S%d  modify logs, whicih will to be modified index = %d,firstlog index = %d at T%d", time.Now().Unix()%10000, dLeader, rf.me, i, firstLog.Index, rf.currentTerm)
		index := log.Index
		term := log.Term
		// DPrintf("%d %s  S%d  modify logs, situation is %v || %v at T%d", time.Now().Unix()%10000, dLeader, rf.me, index > lastLog.Index, rf.log[index-firstLog.Index].Term != term, rf.currentTerm)
		DPrintf("%d %s S%d index=%d, lastLog.Index=%d, firstLog.Index=%d, term=%d at T%d", time.Now().Unix()%10000, dInfo, rf.me, index, lastLog.Index, firstLog.Index, term, rf.currentTerm)
		if index > lastLog.Index || rf.log[index-firstLog.Index].Term != term {
			var tmp []Entry

			rf.log = append(tmp, rf.log[:index-firstLog.Index]...)
			return i
		}
	}

	return len(logs) // 不是len-1是因为如果是这个，那就是最后一个有问题
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
	rf.heartBeatInterval = 10 * time.Millisecond //10 * time.Millisecond
	rf.heartBeatTime = time.NewTimer(rf.heartBeatInterval)
	rf.heartBeatTime.Stop()
	rf.electionTime = time.NewTimer(rf.GetRandomElectionTime())
	rf.log = make([]Entry, 1)
	rf.votedFor = -1
	rf.identity = FOLLOWER
	rf.currentTerm = 1
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.applyCh = applyCh
	rf.nextIndex = make([]int, len(rf.peers))
	rf.matchIndex = make([]int, len(rf.peers))
	for i := 0; i < len(rf.peers); i++ {
		rf.nextIndex[i] = 1
		rf.matchIndex[i] = 0
	}
	// Your initialization code here (3A, 3B, 3C).
	rf.applyCond = *sync.NewCond(&rf.mu)
	DPrintf("%d %s  S%d  has been initialed at T%d", time.Now().Unix()%10000, dLeader, rf.me, rf.currentTerm)
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	DPrintf("server %d is inited\n", rf.me)
	// start ticker goroutine to start elections
	go rf.ticker()
	// go rf.heartBeatTicker()
	go rf.applyChticker()
	return rf
}

func (rf *Raft) applyChticker() {

	for rf.killed() == false {

		rf.mu.Lock()
		if rf.commitIndex <= rf.lastApplied {
			rf.applyCond.Wait()
		}
		firstlog := rf.GetFirstlog()
		reloadLogs := make([]Entry, rf.commitIndex-rf.lastApplied)
		copy(reloadLogs, rf.log[rf.lastApplied+1-firstlog.Index:rf.commitIndex-firstlog.Index+1])
		rf.mu.Unlock()
		// 这里阻塞了，没有提交上去
		for _, entry := range reloadLogs {
			msg := ApplyMsg{
				CommandValid: true,
				CommandIndex: entry.Index,
				Command:      entry.Command,
			}
			rf.applyCh <- msg
			DPrintf("%d %s  S%d applied msg[index = %v] at T%d", time.Now().Unix()%10000, dInfo, rf.me, entry, rf.currentTerm)
		}

		rf.mu.Lock()
		if rf.lastApplied < rf.commitIndex {
			rf.lastApplied = rf.commitIndex
		}
		rf.mu.Unlock()
	}

}

type logTopic string

const (
	dClient  logTopic = "CLNT"
	dCommit  logTopic = "CMIT"
	dDrop    logTopic = "DROP"
	dError   logTopic = "ERRO"
	dInfo    logTopic = "INFO"
	dLeader  logTopic = "LEAD"
	dLog     logTopic = "LOG1"
	dLog2    logTopic = "LOG2"
	dPersist logTopic = "PERS"
	dSnap    logTopic = "SNAP"
	dTerm    logTopic = "TERM"
	dTest    logTopic = "TEST"
	dTimer   logTopic = "TIMR"
	dTrace   logTopic = "TRCE"
	dVote    logTopic = "VOTE"
	dWarn    logTopic = "WARN"
)

type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []Entry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool

	// 添加这两个，因为单靠nextIndex一次次自减速度太慢了，对于长时间挂机的来说无法达到恢复日志的目的
	ConflictIndex int
	ConflictTerm  int
}
