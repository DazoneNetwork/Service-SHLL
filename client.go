package main

import (
    "fmt"
    "os/exec"
	"flag"
	"sort"
	"strconv"
	"strings"
	docker "github.com/fsouza/go-dockerclient"
)
type node struct{
    name string
    ip string
    usage [] string
    status string
    weight int
}
func main(){
    /*
    var netname string // docker 单机网络名称
    var ip string // 服务的虚拟IP
    var port string // 服务端口
    var image string // 服务镜像
    var protocol string // 服务协议 tcp udp
    */
    var usage [] string
    // var service map[string]node
    service := make(map[string]*node)
    total := 0
    
	client, err := docker.NewClientFromEnv()
	if err != nil {
		panic(err)
	}
	
	imgs, err := client.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		panic(err)
	}
	//var cstats chan<- *docker.Stats
    netname := flag.String("n", "mynet", "net")    //网络名称
    replicate := flag.Int("r", 3, "replicate num")           //副本数
    image := flag.String("m", "centos:latest", "image mirror")//镜像
    startcommand := flag.String("s", "nginx", "start command")
    createcommand := flag.String("c", "", "create command")
    mount := flag.String("v", "", "mount dir")
    init := flag.String("a", "", "init")
    ip := flag.String("i", "172.16.19.103", "ip")           //虚拟IP
    flag.Parse()
    if *init !=""{
        exec.Command("/bin/sh", "-c", "ipvsadm -C").Output()
        //o,_ := c.Output()
        fmt.Println(*image)
        exec.Command("/bin/sh", "-c", "ipvsadm -A -t " + *ip + ":80 -s wrr").Output()
        exec.Command("/bin/sh", "-c", "ipvsadm -A -t " + *ip + ":443 -s wrr").Output()
    }
    /*port := flag.String("p", "80", "port")//端口
    protocol := flag.String("t", "tcp", "protocol")//镜像
    */ 
	for _, img := range imgs {
	    net, ok := img.Networks.Networks[*netname] 
	    if img.Image != *image{
	        continue
	    }
	    //fmt.Println(replicate, total+1)
        if *replicate < total+1 && ok{
            // fmt.Println(replicate, total)
    	    // 关闭超过的container
    	    cmd := exec.Command("/bin/sh", "-c", "docker stop " + img.ID)
        	_, err := cmd.Output()
        	if err == nil{
        	    
            	exec.Command("/bin/bash", "-c", "ipvsadm -d -t " + *ip + ":80 -r " + net.IPAddress + ":80 -g").Output()
                exec.Command("/bin/bash", "-c", "ipvsadm -d -t " + *ip + ":443 -r " + net.IPAddress + ":443 -g").Output()
        	    delete(service, img.ID)
        	}
    	    continue   
    	}
	    
	    // fmt.Println("%s", img.Names)
	    // fmt.Println("%s", img.Image)
	    if img.State == "running" && ok{
	        // fmt.Println(net.IPAddress) // 存活检测
            cmd := exec.Command("/bin/sh", "-c", "docker stats " + img.ID + " --no-stream --format \"table {{.CPUPerc}}|{{.MemPerc}}\"")
        	buf, err := cmd.Output()
        	if err == nil{ //\n
        	    usage = strings.Split(strings.Split(strings.Replace(strings.Replace(string(buf), "\n", "", -1), "%", "", -1),"MEM ")[1],"|")
        	    // fmt.Printf("output: %s\n", usage[0])
                // fmt.Println(usage)
        	    total += 1
        	    _, ok = service[img.ID]
            	if ok{
                    service[img.ID].usage = usage
            	}else{ // 没创建
    	            service[img.ID] = &node{
    	                name: img.Names[0], 
    	                ip: net.IPAddress, 
    	                usage: usage, 
    	                weight: 10}
            	}
        	}
        	
        	
	         // 存活检测
        }else{
           // fmt.Println(img.Image == image)
           if img.Image == *image{
                cmd := exec.Command("/bin/sh", "-c", "docker start " + img.ID)
        	    _, err := cmd.Output()
        	    fmt.Println("docker start " + img.ID)
            	if err == nil{
                    exec.Command("/bin/bash", "-c", "docker exec " + img.ID + " echo 2 > /proc/sys/net/ipv4/conf/all/arp_announce").Output()
                    exec.Command("/bin/bash", "-c", "docker exec " + img.ID + " echo 2 > /proc/sys/net/ipv4/conf/lo/arp_announce").Output()
                    exec.Command("/bin/bash", "-c", "docker exec " + img.ID + " echo 1 > /proc/sys/net/ipv4/conf/all/arp_ignore").Output()
                    exec.Command("/bin/bash", "-c", "docker exec " + img.ID + " echo 1 > /proc/sys/net/ipv4/conf/lo/arp_ignore").Output()
                    exec.Command("/bin/bash", "-c", "docker exec " + img.ID + " ip addr add " + *ip + "/32 dev lo label lo:1").Output()
                    exec.Command("/bin/bash", "-c", "docker exec " + img.ID + " "+ *startcommand).Output()
                    
            	    ipc := exec.Command("/bin/sh", "-c", "docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}'" + img.ID)
            	    vip, err := ipc.Output()
            	    if err == nil{
            	        //fmt.Println(ip)
                    	_, ok = service[img.ID]
                    	if ok{
                            service[img.ID].usage = []string{"0.00", "0.00"}
                    	}else{
            	            service[img.ID] = &node{name: img.Names[0], ip: string(vip), usage: []string{"0.00", "0.00"}, weight: 10}
                    	}
                        total += 1  
            	    }
            	    // 添加服务器      	    
            	}else{
            	    // 启动失败 删除service
            	    //exec.Command("/bin/bash", "-c", "ipvsadm -d -t " + *ip + ":80 -r " + service[img.ID].ip + ":80 -g").Output()
            	    //exec.Command("/bin/bash", "-c", "ipvsadm -d -t " + *ip + ":443 -r " + service[img.ID].ip + ":443 -g").Output()
            	    delete(service, img.ID)
            	    
            	}
            }
	
	    }
    	
	}
	if *replicate > total{
        //启动更多的container
        should := *replicate
        for i := total; i < should; i++ {
	        // fmt.Println( replicate, total )
            c := exec.Command("/bin/bash", "-c", "docker run --privileged -itd " + *mount + " --network=" + *netname + " " + *image + " " + *createcommand)
            bo, err := c.Output()
            if err == nil {
                out := strings.Replace(string(bo),"\n","",-1)
                exec.Command("/bin/bash", "-c", "docker exec " + out + " echo 2 > /proc/sys/net/ipv4/conf/all/arp_announce").Output()
                exec.Command("/bin/bash", "-c", "docker exec " + out + " echo 2 > /proc/sys/net/ipv4/conf/lo/arp_announce").Output()
                exec.Command("/bin/bash", "-c", "docker exec " + out + " echo 1 > /proc/sys/net/ipv4/conf/all/arp_ignore").Output()
                exec.Command("/bin/bash", "-c", "docker exec " + out + " echo 1 > /proc/sys/net/ipv4/conf/lo/arp_ignore").Output()
                exec.Command("/bin/bash", "-c", "docker exec " + out + " ip addr add " + *ip + "/32 dev lo label lo:1").Output()
                exec.Command("/bin/bash", "-c", "docker exec " + out + " "+ *startcommand).Output()
                
                // fmt.Println("docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}'" + out)
            	ipc := exec.Command("/bin/sh", "-c", "docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}'" + out)
            	vip, err := ipc.Output()
            	fmt.Println(vip)
            	if err == nil{
                    _, ok := service[out]
                	if ok{
                	    service[out].usage = []string{"0.00", "0.00"}
                	}else{
                        service[out] = &node{name: "", ip: string(vip), usage: []string{"0.00", "0.00"}, weight: 10}
                	}
                    total += 1  
        	    }
            }
        }
    } 
	// ipvs 处理
	var cpu []float64
	var mem []float64
	for _, value := range service{
	    // fmt.Println("cpu")
	    // fmt.Println(strconv.ParseFloat(value.usage[1], 64))
	    a,_ := strconv.ParseFloat(value.usage[0], 64)
	    cpu = append(cpu, a)
	    b,_ := strconv.ParseFloat(value.usage[1], 64)
	    mem = append(mem, b)
	    // fmt.Println(value.usage)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(cpu)))
	sort.Sort(sort.Reverse(sort.Float64Slice(mem)))
	fmt.Println(cpu, mem)
	for key, value := range service{
	    wa := 0
	    wb := 0
	    for i := 0; i < len(cpu); i++ {
	        a,_ := strconv.ParseFloat(value.usage[0], 64);
	        if a == cpu[i]{
	            wa = i+1
	            break
	        }
	    }
	    for i := 0; i < len(mem); i++ {
	        a,_ := strconv.ParseFloat(value.usage[1], 64);
	        if a == mem[i]{
	            wb = i+1
	            break
	        }
	    }
        // fmt.Println(wa, wb)
	    // wa := sort.Search(len(cpu), func(i int) bool { a,_ := strconv.ParseFloat(value.usage[0], 64);return cpu[i]==a})
	    // wb := sort.Search(len(mem), func(i int) bool { a,_ := strconv.ParseFloat(value.usage[1], 64);return mem[i]==a})
	    service[key].weight = (wa+wb+2)/2
	    exec.Command("/bin/bash", "-c", "ipvsadm -d -t " + *ip + ":80 -r " + service[key].ip + ":80 -g").Output()
	    exec.Command("/bin/bash", "-c", "ipvsadm -d -t " + *ip + ":443 -r " + service[key].ip + ":443 -g").Output()
	    exec.Command("/bin/bash", "-c", "ipvsadm -a -t " + *ip + ":80 -r " + service[key].ip + ":80 -w " + strconv.Itoa(service[key].weight) + " -g").Output()
	    exec.Command("/bin/bash", "-c", "ipvsadm -a -t " + *ip + ":443 -r " + service[key].ip + ":443 -w " + strconv.Itoa(service[key].weight) + " -g").Output()
	    // fmt.Println(service[key].weight)
	}
	
    exec.Command("/bin/sh", "-c", "ipvsadm -S").Output()
    	    /*for net := range img.Networks.Networks {
    		    fmt.Println(net, ":", img.Networks.Networks[net].IPAddress)
    		    break
    	    }*/
	        
	    /*
		fmt.Println("ID: ", img.ID)
		fmt.Println("Name: ", img.Names[0])
		fmt.Println("State: ", img.State)
		fmt.Println("Status: ", img.Status)
		fmt.Println("Networks: ", img.Networks[0])
		fmt.Println("Image: ", img.Image)*/
    
}
