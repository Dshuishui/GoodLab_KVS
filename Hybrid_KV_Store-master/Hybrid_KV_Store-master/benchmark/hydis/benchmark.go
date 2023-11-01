package main

import (
	"flag"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	kvc "github.com/JasonLou99/Hybrid_KV_Store/kvstore/kvclient"
	"github.com/JasonLou99/Hybrid_KV_Store/util"
)

const (
	CAUSAL = iota
	BoundedStaleness
)

var count int32 = 0
var putCount int32 = 0
var getCount int32 = 0
var falseTime int32 = 0

// Test the consistency performance at different read/write ratios
func RequestRatio(cnum int, num int, servers []string, getRatio int, consistencyLevel int, quorum int) {
	fmt.Printf("servers: %v\n", servers)
	kvc := kvc.KVClient{
		Kvservers:   make([]string, len(servers)),
		Vectorclock: make(map[string]int32),
	}
	kvc.ConsistencyLevel = CAUSAL
	for _, server := range servers {
		kvc.Vectorclock[server+"1"] = 0
	}
	copy(kvc.Kvservers, servers)
	start_time := time.Now()
	for i := 0; i < num; i++ {
		rand.Seed(time.Now().UnixNano())
		key := rand.Intn(100)
		value := rand.Intn(100000)
		startTime := time.Now().UnixMicro()
		// 写操作
		kvc.PutInCausal("key"+strconv.Itoa(key), "value"+strconv.Itoa(value))
		spentTime := int(time.Now().UnixMicro() - startTime)
		// util.DPrintf("%v", spentTime)
		kvc.PutSpentTimeArr = append(kvc.PutSpentTimeArr, spentTime)
		atomic.AddInt32(&putCount, 1)
		atomic.AddInt32(&count, 1)
		// util.DPrintf("put success")
		for j := 0; j < getRatio; j++ {
			// 读操作
			k := "key" + strconv.Itoa(key)
			var v string
			if quorum == 1 {
				v, _ = kvc.GetInCausalWithQuorum(k)
			} else {
				v, _ = kvc.GetInCausal(k)
			}
			// if GetInCausal return, it must be success
			atomic.AddInt32(&getCount, 1)
			atomic.AddInt32(&count, 1)
			if v != "" {
				// 查询出了值就输出，屏蔽请求非Leader的情况
				// util.DPrintf("TestCount: ", count, ",Get ", k, ": ", ck.Get(k))
				util.DPrintf("TestCount: %v ,Get %v: %v, VectorClock: %v, getCount: %v, putCount: %v", count, k, v, kvc.Vectorclock, getCount, putCount)
				// util.DPrintf("spent: %v", time.Since(start_time))
			}
		}
		// 随机切换下一个节点
		kvc.KvsId = rand.Intn(len(kvc.Kvservers)+10) % len(kvc.Kvservers)
	}
	fmt.Printf("TestCount: %v, VectorClock: %v, getCount: %v, putCount: %v\n", count, kvc.Vectorclock, getCount, putCount)
	if int(count) == num*cnum*(getRatio+1) {
		fmt.Printf("Task is completed, spent: %v\n", time.Since(start_time))
		fmt.Printf("falseTimes: %v\n", falseTime)
	}
	util.WriteCsv("./benchmark/result/causal_put-latency.csv", kvc.PutSpentTimeArr)
}

/*
	根据csv文件中的读写频次发送请求
*/
func benchmarkFromCSV(filepath string, servers []string, clientNumber int) {

	kvc := kvc.KVClient{
		Kvservers:   make([]string, len(servers)),
		Vectorclock: make(map[string]int32),
	}
	for _, server := range servers {
		kvc.Vectorclock[server+"1"] = 0
	}
	copy(kvc.Kvservers, servers)
	writeCounts := util.ReadCsv(filepath)
	start_time := time.Now()
	for i := 0; i < len(writeCounts); i++ {
		for j := 0; j < int(writeCounts[i]); j++ {
			// 重复发送写请求
			startTime := time.Now().UnixMicro()
			// kvc.PutInWritelessCausal("key"+strconv.Itoa(i), "value"+strconv.Itoa(i+j))
			kvc.PutInWritelessCausal("key", "value"+strconv.Itoa(i+j))
			spentTime := int(time.Now().UnixMicro() - startTime)
			kvc.PutSpentTimeArr = append(kvc.PutSpentTimeArr, spentTime)
			util.DPrintf("%v", spentTime)
			atomic.AddInt32(&putCount, 1)
			atomic.AddInt32(&count, 1)
		}
		// 发送一次读请求
		// v, _ := kvc.GetInWritelessCausal("key" + strconv.Itoa(i))
		v, _ := kvc.GetInWritelessCausal("key")
		atomic.AddInt32(&getCount, 1)
		atomic.AddInt32(&count, 1)
		if v != "" {
			// 查询出了值就输出，屏蔽请求非Leader的情况
			// util.DPrintf("TestCount: ", count, ",Get ", k, ": ", ck.Get(k))
			util.DPrintf("TestCount: %v ,Get %v: %v, VectorClock: %v, getCount: %v, putCount: %v", count, "key"+strconv.Itoa(i), v, kvc.Vectorclock, getCount, putCount)
			// util.DPrintf("spent: %v", time.Since(start_time))
		}
		// 随机切换下一个节点
		kvc.KvsId = rand.Intn(len(kvc.Kvservers)+10) % len(kvc.Kvservers)
	}
	fmt.Printf("Task is completed, spent: %v. TestCount: %v, getCount: %v, putCount: %v \n", time.Since(start_time), count, getCount, putCount)
	// 时延数据写入csv
	util.WriteCsv("./benchmark/result/writless-causal_put-latency"+strconv.Itoa(clientNumber)+".csv", kvc.PutSpentTimeArr)
}

// zipf分布 高争用情况 zipf=x
func zipfDistributionBenchmark(x int, n int) int {
	return 0
}
func main() {
	var ser = flag.String("servers", "", "the Server, Client Connects to")
	// var mode = flag.String("mode", "RequestRatio", "Read or Put and so on")
	var mode = flag.String("mode", "BenchmarkFromCSV", "Read or Put and so on")
	var cnums = flag.String("cnums", "1", "Client Threads Number")
	var onums = flag.String("onums", "1", "Client Requests Times")
	var getratio = flag.String("getratio", "1", "Get Times per Put Times")
	var cLevel = flag.Int("consistencyLevel", CAUSAL, "Consistency Level")
	var quorumArg = flag.Int("quorum", 0, "Quorum Read")
	flag.Parse()
	servers := strings.Split(*ser, ",")
	clientNumm, _ := strconv.Atoi(*cnums)
	optionNumm, _ := strconv.Atoi(*onums)
	getRatio, _ := strconv.Atoi(*getratio)
	consistencyLevel := int(*cLevel)
	quorum := int(*quorumArg)

	if clientNumm == 0 {
		fmt.Println("### Don't forget input -cnum's value ! ###")
		return
	}
	if optionNumm == 0 {
		fmt.Println("### Don't forget input -onumm's value ! ###")
		return
	}

	// Request Times = clientNumm * optionNumm
	if *mode == "RequestRatio" {
		for i := 0; i < clientNumm; i++ {
			go RequestRatio(clientNumm, optionNumm, servers, getRatio, consistencyLevel, quorum)
		}
	} else if *mode == "BenchmarkFromCSV" {
		for i := 0; i < clientNumm; i++ {
			go benchmarkFromCSV("./writeless/dataset/btcusd_low.csv", servers, clientNumm)
		}
	} else {
		fmt.Println("### Wrong Mode ! ###")
		return
	}
	// fmt.Println("count")
	// time.Sleep(time.Second * 3)
	// fmt.Println(count)
	//	return

	// keep main thread alive
	time.Sleep(time.Second * 1200)
}
