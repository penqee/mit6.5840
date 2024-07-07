package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"sort"
	"time"
)

// for sorting by key.
type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

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

	// Your worker implementation here.

	// uncomment to send the Example RPC to the coordinator.
	// CallExample()
	for {
		task := RequestTask()

		switch task.Type {
		case MAP:
			DPrintln("map task ", task.FileName)
			dealWithMapTask(&task, mapf)
			TaskDone(task.ID)

		case REDUCE:
			DPrintln("reduce task")
			dealWithReduceTask(&task, reducef)
			TaskDone(task.ID)

		case NO_TASK:
			break
		case QUIT:
			TaskDone(task.ID)
			time.Sleep(time.Second)
			os.Exit(0)
		default:
			DPrintln("unknown task")

		}

		time.Sleep(100 * time.Millisecond)
	}

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

func RequestTask() Task {
	args := RequestTaskArgs{}
	reply := RequestTaskReply{}

	ok := call("Coordinator.RequestTask", &args, &reply)
	if ok {
		DPrintf("get Task %s successfully\n", reply.Task.Type)
	} else {
		DPrintf("failed to get Task\n")
	}

	return reply.Task
}

func TaskDone(id int) {
	args := TaskDoneArgs{id}
	reply := TaskDoneReply{}

	ok := call("Coordinator.TaskDone", &args, &reply)
	if ok {
		DPrintf("send Task Done successfully\n")
	} else {
		DPrintf("failed to send Task Done \n")
	}

}

func dealWithMapTask(task *Task, mapf func(string, string) []KeyValue) {
	filename := task.FileName

	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("can not open %v", filename)
	}
	// 读取文件内容
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("can not read %v", filename)
	}
	file.Close()
	// 执行map函数 得到kv对

	kva := mapf(filename, string(content))
	buckets := make(map[int][]KeyValue)
	nReduce := task.NReduce
	// 根据不同的key 放入不同的bucket
	for _, kv := range kva {
		index := ihash(kv.Key) % nReduce
		buckets[index] = append(buckets[index], kv)
	}

	// 把不同bucket的内容写入文件中去
	for i := 0; i < nReduce; i++ {
		rFile, err := os.Create(getTempFileName(task.ID, i))
		if err != nil {
			log.Println("failed to create file")
		}
		enc := json.NewEncoder(rFile)
		kvs := buckets[i]

		for _, kv := range kvs {
			if err := enc.Encode(kv); err != nil {
				log.Println("failed to encode kv ,err:", err)
			}
		}

	}
}

func dealWithReduceTask(task *Task, reducef func(string, []string) string) {
	nMap := task.NMap
	intermediate := make([]KeyValue, 0)

	for i := 0; i < nMap; i++ {
		rFile, err := os.Open(getTempFileName(i, task.ID))
		if err != nil {
			log.Println("failed to open file")
		}

		dec := json.NewDecoder(rFile)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			intermediate = append(intermediate, kv)
		}
	}

	sort.Sort(ByKey(intermediate))

	// 创建输出文件
	ofile, _ := os.Create(getOutputFileName(task.ID))

	//
	// call Reduce on each distinct key in intermediate[],
	// and print the result to mr-out-0.
	//
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

	ofile.Close()

	// 删除文件
	for i := 0; i < nMap; i++ {
		os.Remove(getTempFileName(i, task.ID))
	}

}

func getTempFileName(mapNumber int, reduceNumber int) string {
	return fmt.Sprintf("mr-%d-%d", mapNumber, reduceNumber)
}

func getOutputFileName(reduceNumber int) string {
	return fmt.Sprintf("mr-out-%d", reduceNumber)
}
