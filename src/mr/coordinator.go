package mr

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sort"
	"sync"
	"time"
)

const DEBUG = false

func DebugPrintln(items ...interface{}) {
	if DEBUG {
		log.Println(items)
	}
}

type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

const (
	MAP      = "MAP"
	REDUCE   = "REDUCE"
	NONETASK = "NONETASK"
	QUIT     = "QUIT"
	TIMEOUT  = 10 * time.Second

	//Deadline
	NOALLOCATION = -1
)

type CoordinatorPhase string

type Coordinator struct {
	// Your definitions here.
	TaskChannel chan *Task
	TaskMap     map[int]*Task
	Stage       CoordinatorPhase
	NMap        int
	NReduce     int
	Mutex       sync.Mutex
}

type Task struct {
	Tid      int
	File     string
	Status   string
	Deadline int
	NReduce  int
	NMap     int
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
	// ret := false

	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	return c.Stage == QUIT

	// Your code here.

	// return ret
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{
		TaskChannel: make(chan *Task, int(math.Max(float64(len(files)), float64(nReduce)))),
		TaskMap:     make(map[int]*Task),
		Stage:       MAP,
		NMap:        len(files),
		NReduce:     nReduce,
		Mutex:       sync.Mutex{},
	}

	for i := 0; i < len(files); i++ {
		c.DoMapTask(files[i], i)
	}

	// Your code here.

	go c.DetectError()

	c.server()
	return &c
}

func (c *Coordinator) DetectError() {
	for {
		c.Mutex.Lock()
		if len(c.TaskMap) != 0 {
			for _, task := range c.TaskMap {

				if task.Deadline < int(time.Now().Unix()) && task.Deadline != NOALLOCATION {
					//at this time, the task go wrong
					task.Deadline = NOALLOCATION
					c.TaskChannel <- task
				}

			}
		} else {
			c.ConvertStatus()
		}
		c.Mutex.Unlock()
	}
}

func (c *Coordinator) ConvertStatus() {
	switch c.Stage {
	case MAP:
		c.Stage = REDUCE
		for i := 0; i < c.NReduce; i++ {
			c.DoReduceTask("", i)
		}
		DebugPrintln("map convert to reduce")
	case REDUCE:
		c.Stage = QUIT
		for i := 0; i < c.NReduce; i++ {
			c.DoQuitTask(i)
		}

	case QUIT:
		DebugPrintln("debug coordinator quit")
		os.Exit(0)
	default:
		DebugPrintln("unknown status")

	}
}

func (c *Coordinator) DoQuitTask(index int) {
	task := &Task{
		Tid:      index,
		Status:   QUIT,
		Deadline: NOALLOCATION,
		NReduce:  c.NReduce,
		NMap:     c.NMap,
	}
	c.TaskChannel <- task
	c.TaskMap[index] = task
}

func (c *Coordinator) DoMapTask(file string, index int) {
	task := &Task{
		Tid:      index,
		File:     file,
		Status:   MAP,
		Deadline: NOALLOCATION,
		NReduce:  c.NReduce,
		NMap:     c.NMap,
	}
	c.TaskChannel <- task
	c.TaskMap[index] = task
}

func (c *Coordinator) DoReduceTask(file string, index int) {
	task := &Task{
		Tid:      index,
		File:     file,
		Status:   REDUCE,
		Deadline: NOALLOCATION,
		NReduce:  c.NReduce,
		NMap:     c.NMap,
	}
	c.TaskChannel <- task
	c.TaskMap[index] = task
}

func (c *Coordinator) TaskRequest(args *TaskRequestReq, reply *TaskResponseResp) error {
	var task *Task
	if len(c.TaskMap) != 0 {
		task = <-c.TaskChannel
		task.Deadline = int(time.Now().Add(TIMEOUT).Unix())
		reply.Task = task
		return nil
	}
	task.Status = NONETASK
	reply = &TaskResponseResp{Task: task}
	return nil

}

func DealWithMapTask(mapf func(string, string) []KeyValue, task *Task) {
	intermediate := []KeyValue{}
	file, err := os.Open(task.File)
	if err != nil {
		DebugPrintln("failed to open the file", task.File)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		DebugPrintln("failed to read the file", task.File)
	}
	file.Close()
	kva := mapf(task.File, string(content))
	intermediate = append(intermediate, kva...)

	for i := 0; i < task.NReduce; i++ {
		file, err := os.Create(GetTempFile(task.Tid, i))
		if err != nil {
			DebugPrintln("failed to create file")
		}
		encoder := json.NewEncoder(file)

		for _, kv := range intermediate {
			if ihash(kv.Key)%task.NReduce == i {
				err = encoder.Encode(&kv)
				if err != nil {
					DebugPrintln("failed to encode")
				}
			}
		}
		file.Close()
	}

}

func DealWithReduceTask(reducef func(string, []string) string, task *Task) {
	intermediate := []KeyValue{}
	for i := 0; i < task.NReduce; i++ {
		file, err := os.Open(GetTempFile(task.Tid, i))
		if err != nil {
			DebugPrintln("failed to open the file", err)
		}
		decoder := json.NewDecoder(file)
		var kv []KeyValue
		if err = decoder.Decode(&kv); err != nil {
			DebugPrintln("failed to read the file", err)
		}
		intermediate = append(intermediate, kv...)
		file.Close()
	}

	sort.Sort(ByKey(intermediate))
	ofile, err := os.Create(GetFinalFile(task.Tid))
	if err != nil {
		DebugPrintln("failed to create file", err)
	}

	i := 0
	for i < len(intermediate) {
		j := i + 1
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Value)
		}
		output := reducef(intermediate[i].Key, values)

		// this is the correct format for each line of Reduce output.
		fmt.Fprintf(ofile, "%v %v\n", intermediate[i].Key, output)

		i = j
	}

	for i := 0; i < task.NReduce; i++ {
		err = os.Remove(GetTempFile(task.Tid, i))
		if err != nil {
			DebugPrintln("failed to remove")
		}
	}
	ofile.Close()
}

func (c *Coordinator) TaskDone(args *TaskDoneRequest, reply *TaskDoneResponse) error {
	DebugPrintln("taskDone!")
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	delete(c.TaskMap, args.Tid)
	return nil
}

func GetTempFile(x, y int) string {
	return fmt.Sprintf("mr-%d-%d", x, y)
}

func GetFinalFile(x int) string {
	return fmt.Sprintf("mr-out-%d", x)
}
