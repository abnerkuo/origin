package node

import (
	"fmt"
	"github.com/duanhf2012/origin/cluster"
	"github.com/duanhf2012/origin/console"
	"github.com/duanhf2012/origin/log"
	"github.com/duanhf2012/origin/profiler"
	"github.com/duanhf2012/origin/service"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var closeSig chan bool
var sigs chan os.Signal
var nodeId int
var preSetupService []service.IService //预安装
var profilerInterval time.Duration

func init() {
	closeSig = make(chan bool,1)
	sigs = make(chan os.Signal, 3)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM,syscall.Signal(10))
}


func  getRunProcessPid() (int,error) {
	f, err := os.OpenFile(os.Args[0]+".pid", os.O_RDONLY, 0600)
	defer f.Close()
	if err!= nil {
		return 0,err
	}

	pidbyte,errs := ioutil.ReadAll(f)
	if errs!=nil {
		return 0,errs
	}

	return strconv.Atoi(string(pidbyte))
}

func writeProcessPid() {
	//pid
	f, err := os.OpenFile(os.Args[0]+".pid", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0600)
	defer f.Close()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(-1)
	} else {
		_,err=f.Write([]byte(fmt.Sprintf("%d",os.Getpid())))
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(-1)
		}
	}
}

func GetNodeId() int {
	return nodeId
}

func initNode(id int){
	//1.初始化集群
	nodeId = id
	err := cluster.GetCluster().Init(GetNodeId())
	if err != nil {
		log.Fatal("read system config is error %+v",err)
	}

	//2.setup service
	for _,s := range preSetupService {
		//是否配置的service
		if cluster.GetCluster().IsConfigService(s.GetName()) == false {
			continue
		}

		pServiceCfg := cluster.GetCluster().GetServiceCfg(nodeId,s.GetName())
		s.Init(s,cluster.GetRpcClient,cluster.GetRpcServer,pServiceCfg)

		service.Setup(s)
	}

	//3.service初始化
	service.Init(closeSig)
}

func Start() {
	console.RegisterCommand("start",startNode)
	console.RegisterCommand("stop",stopNode)
	err := console.Run(os.Args)
	if err!=nil {
		fmt.Printf("%+v\n",err)
		return
	}
}


func stopNode(args []string) error {
	processid,err := getRunProcessPid()
	if err != nil {
		return err
	}

	KillProcess(processid)
	return nil
}

func startNode(args []string) error {
	//1.解析参数
	param := args[2]
	sparam := strings.Split(param,"=")
	if len(sparam) != 2 {
		return fmt.Errorf("invalid option %s",param)
	}
	if sparam[0]!="nodeid" {
		return fmt.Errorf("invalid option %s",param)
	}
	nodeId,err:= strconv.Atoi(sparam[1])
	if err != nil {
		return fmt.Errorf("invalid option %s",param)
	}

	log.Release("Start running server.")
	//2.初始化node
	initNode(nodeId)

	//3.运行集群
	cluster.GetCluster().Start()

	//4.运行service
	service.Start()

	//5.记录进程id号
	writeProcessPid()

	//6.监听程序退出信号&性能报告
	bRun := true
	var pProfilerTicker *time.Ticker = &time.Ticker{}
	if profilerInterval>0 {
		pProfilerTicker = time.NewTicker(profilerInterval)
	}
	for bRun {
		select {
		case <-sigs:
			log.Debug("receipt stop signal.")
			bRun = false
		case <- pProfilerTicker.C:
			profiler.Report()
		}
	}

	//7.退出
	close(closeSig)
	service.WaitStop()

	log.Debug("Server is stop.")
	return nil
}


func Setup(s ...service.IService)  {
	for _,sv := range s {
		sv.OnSetup(sv)
		preSetupService = append(preSetupService,sv)
	}
}

func GetService(servicename string) service.IService {
	return service.GetService(servicename)
}

func SetConfigDir(configdir string){
	cluster.SetConfigDir(configdir)
}

func SetSysLog(strLevel string, pathname string, flag int){
	logs,_:= log.New(strLevel,pathname,flag)
	log.Export(logs)
}

func OpenProfilerReport(interval time.Duration){
	profilerInterval = interval
}
