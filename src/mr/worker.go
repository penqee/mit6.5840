package mr

import (
	"fmt"
	"hash/fnv"
	"log"
	"net/rpc"
	"os"
	"time"
)

// Map functions return a slice of KeyValue.
type KeyValue struct {
	Key   string
	Value string
}

// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// main/mrworker.go calls this function.
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	for {
		req := CallTaskRequest()
		switch req.Status {
		case MAP:
			DealWithMapTask(mapf, req)
			CallTaskDone(req.Tid)
		case REDUCE:
			DealWithReduceTask(reducef, req)
			CallTaskDone(req.Tid)
		case QUIT:
			CallTaskDone(req.Tid)
			time.Sleep(time.Second)
			os.Exit(0)
		case NONETASK:

		default:
			DebugPrintln("unknown task")
		}
	}

	// switch {

	// case MAP:
	// 	DealWithMapTask(mapf, req)

	// case REDUCE:
	// 	DealWithReduceTask(reducef, req)

	// case NONETASK:

	// case QUIT:

	// default:
	// 	DebugPrintln("undefined task")
	// }
}

// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		fmt.Printf("call failed!\n")
	}
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}

func CallTaskRequest() *Task {
	args := &TaskRequestReq{}
	reply := &TaskResponseResp{}

	ok := call("Coordinator.TaskRequest", args, reply)
	if ok {
		DebugPrintln("reply = ", reply)
	} else {
		DebugPrintln("call failed\n")
	}

	return reply.Task
}

func CallTaskDone(tid int) {
	args := &TaskDoneRequest{Tid: tid}
	reply := &TaskDoneResponse{}

	ok := call("Coordinator.TaskDone", args, reply)
	if ok {
		DebugPrintln("successfully call")
	} else {
		DebugPrintln("failed to call")
	}

}
