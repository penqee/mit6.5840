package mr

import (
	"log"
	"math"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)

const Debug = false

func DPrintln(a ...interface{}) {
	if Debug {
		log.Println(a...)
	}
}

func DPrintf(format string, a ...interface{}) {
	if Debug {
		log.Printf(format, a...)
	}
}

type Coordinator struct {
	// Your definitions here.
	state       string        // 阶段
	nReduce     int           // reduce数量
	nMap        int           // map数量
	taskChannel chan *Task    // 通道
	taskMap     map[int]*Task // 任务列表
	mutex       sync.Mutex    // 锁
}

type TaskType string

const (
	MAP         = "map"
	REDUCE      = "reduce"
	NO_TASK     = "no_task"
	QUIT        = "quit"
	TIME_OUT    = 10 * time.Second
	UNALLOCATED = -1
)

type Task struct {
	ID       int
	Type     TaskType
	FileName string
	NReduce  int
	NMap     int
	Deadline int64
}

// Your code here -- RPC handlers for the worker to call.

// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	ret := false
	c.mutex.Lock()
	ret = c.state == QUIT
	c.mutex.Unlock()
	// Your code here.

	return ret
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{
		nReduce:     nReduce,
		nMap:        len(files),
		taskChannel: make(chan *Task, int(math.Max(float64(len(files)), float64(nReduce)))),
		taskMap:     make(map[int]*Task),
		mutex:       sync.Mutex{},
		state:       MAP,
	}

	// Your code here.

	for i, filename := range files {

		task := Task{
			ID:       i,
			Type:     MAP,
			FileName: filename,
			NReduce:  c.nReduce,
			NMap:     c.nMap,
			Deadline: UNALLOCATED,
		}
		c.taskChannel <- &task
		c.taskMap[i] = &task
	}
	go c.detector()

	c.server()
	return &c
}

func (c *Coordinator) detector() {
	for {
		c.mutex.Lock()
		DPrintln("current task number ", len(c.taskMap))
		if len(c.taskMap) == 0 {
			c.changeState()
		} else {
			c.taskTimeout()
		}
		c.mutex.Unlock()

		time.Sleep(100 * time.Millisecond)
	}
}

func (c *Coordinator) taskTimeout() {
	for _, task := range c.taskMap {
		DPrintln(time.Now().Unix(), " ", task.Deadline)
		if (task.Deadline != UNALLOCATED) && (time.Now().Unix() > task.Deadline) {
			// 任务超时
			task.Deadline = UNALLOCATED
			c.taskChannel <- task
			DPrintln(task)
		}
	}
}

func (c *Coordinator) changeState() {
	if c.state == MAP {

		c.state = REDUCE
		c.taskMap = make(map[int]*Task)
		for i := 0; i < c.nReduce; i++ {

			task := Task{
				ID:       i,
				Type:     REDUCE,
				NReduce:  c.nReduce,
				NMap:     c.nMap,
				Deadline: UNALLOCATED,
			}
			c.taskChannel <- &task
			c.taskMap[i] = &task
		}
		DPrintln("map convert to reduce")
	} else if c.state == REDUCE {
		c.state = QUIT
		DPrintln("reduce convert to quit")
		for i := 0; i < c.nReduce; i++ {

			task := Task{
				ID:       i,
				Type:     QUIT,
				NReduce:  c.nReduce,
				NMap:     c.nMap,
				Deadline: UNALLOCATED,
			}
			c.taskChannel <- &task
			c.taskMap[i] = &task
		}
	} else if c.state == QUIT {
		log.Println("coordinator exit")
		os.Exit(0)
	}
}

func (c *Coordinator) RequestTask(args *RequestTaskArgs, reply *RequestTaskReply) error {
	if len(c.taskMap) != 0 {
		task := <-c.taskChannel
		task.Deadline = time.Now().Add(TIME_OUT).Unix()
		reply.Task = *task
	} else {
		reply.Task = Task{Type: NO_TASK}
	}
	return nil
}

func (c *Coordinator) TaskDone(args *TaskDoneArgs, reply *TaskDoneReply) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	DPrintln("delete task ", args.ID)
	delete(c.taskMap, args.ID)

	return nil
}
