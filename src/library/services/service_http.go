package services

import (
	log "github.com/sirupsen/logrus"
	"library/http"
	"regexp"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"library/app"
	"encoding/json"
)

// 创建一个新的http服务
func NewHttpService(ctx *app.Context) *HttpService {
	config, _ := getHttpConfig()
	log.Debugf("start http service with config: %+v", config)
	status := serviceDisable
	if !config.Enable {
		return &HttpService{
			status: status,
		}
	}
	status ^= serviceDisable
	status |= serviceEnable
	gc := len(config.Groups)
	client := &HttpService{
		lock:             new(sync.Mutex),
		groups:           make(map[string]*httpGroup, gc),
		sendFailureTimes: int64(0),
		status:           status,
		timeTick:         config.TimeTick,
		wg:               new(sync.WaitGroup),
		ctx:              ctx,
	}
	for _, cgroup := range config.Groups {
		group := &httpGroup{
			name: cgroup.Name,
		}
		group.filter = make([]string, len(cgroup.Filter))
		group.filter = append(group.filter[:0], cgroup.Filter...)

		nc := len(cgroup.Nodes)
		group.nodes = make([]*httpNode, nc)
		for i := 0; i < nc; i++ {
			group.nodes[i] = &httpNode{
				url:              cgroup.Nodes[i],
				sendQueue:        make(chan string, TCP_MAX_SEND_QUEUE),
				sendTimes:        int64(0),
				sendFailureTimes: int64(0),
				lock:             new(sync.Mutex),
				failureTimesFlag: int32(0),
				errorCheckTimes:  int64(0),
				status:           online | cacheNotReady | cacheNotFull,
			}
		}
		client.groups[cgroup.Name] = group
	}

	return client
}

// 开始服务
func (client *HttpService) Start() {
	if client.status & serviceDisable > 0 {
		return
	}
	cpu := runtime.NumCPU()
	for _, cgroup := range client.groups {
		for _, cnode := range cgroup.nodes {
			go client.errorCheckService(cnode)
			// 启用cpu数量的服务协程
			for i := 0; i < cpu; i++ {
				client.wg.Add(1)
				go client.clientSendService(cnode)
			}
		}
	}
}

func (client *HttpService) cacheInit(node *httpNode) {
	if node.status & cacheReady > 0 {
		return
	}
	node.cache = make([][]byte, httpCacheLen)
	for k := 0; k < httpCacheLen; k++ {
		node.cache[k] = nil
	}
	node.status ^= cacheNotReady
	node.status |= cacheReady
	//node.cacheIsInit = true
	node.cacheIndex = 0
	if node.status & cacheFull > 0 {
		//node.cacheFull = false
		node.status ^= cacheFull
		node.status |= cacheNotFull
	}
}

func (client *HttpService) addCache(node *httpNode, msg []byte) {
	log.Debugf("http service add failure cache: %s", node.url)
	node.cache[node.cacheIndex] = append(node.cache[node.cacheIndex][:0], msg...)
	node.cacheIndex++
	if node.cacheIndex >= httpCacheLen {
		node.cacheIndex = 0
		if node.status & cacheNotFull > 0 {
			node.status ^= cacheNotFull
			node.status |= cacheFull
		}
	}
}

func (client *HttpService) sendCache(node *httpNode) {
	if node.cacheIndex > 0 {
		log.Debugf("http service send failure cache: %s", node.url)
		if node.status & cacheFull > 0 {
			for j := node.cacheIndex; j < httpCacheLen; j++ {
				node.sendQueue <- string(node.cache[j])
			}
		}
		for j := 0; j < node.cacheIndex; j++ {
			node.sendQueue <- string(node.cache[j])
		}
		if node.status & cacheFull > 0 {
			node.status ^= cacheFull
			node.status |= cacheNotFull
		}
		node.cacheIndex = 0
	}
}

// 节点故障检测与恢复服务
func (client *HttpService) errorCheckService(node *httpNode) {
	for {
		node.lock.Lock()
		sleepTime := time.Second * client.timeTick
		if node.status & offline > 0 {
			times := atomic.LoadInt64(&node.errorCheckTimes)
			step := float64(times) / float64(1000)
			if step > float64(1) {
				sleepTime = time.Duration(step) * time.Second
				if sleepTime > 60 {
					sleepTime = 60
				}
			}
			// 发送空包检测
			// post默认3秒超时，所以这里不会死锁
			log.Debugf("http服务-故障节点探测：%s", node.url)
			_, err := http.Post(node.url, []byte{byte(0)})
			if err == nil {
				//重新上线
				//node.isDown = false
				node.status ^= offline
				node.status |= online
				atomic.StoreInt64(&node.errorCheckTimes, 0)
				log.Warn("http服务节点恢复", node.url)
				//对失败的cache进行重发
				client.sendCache(node)
			} else {
				log.Errorf("http服务-故障节点发生错误：%+v", err)
			}
			atomic.AddInt64(&node.errorCheckTimes, 1)
		}
		node.lock.Unlock()
		time.Sleep(sleepTime)
		select {
		case <-client.ctx.Ctx.Done():
			log.Debugf("http service %s errorCheckService exit", node.url)
			return
		default:
		}
	}
}

// 节点服务协程
func (client *HttpService) clientSendService(node *httpNode) {
	defer client.wg.Done()
	for {
		select {
		case msg, ok := <-node.sendQueue:
			if !ok {
				log.Warnf("http service, sendQueue channel was closed")
				return
			}
			if node.status & online > 0 {
				atomic.AddInt64(&node.sendTimes, int64(1))
				log.Debugf("http service post to %s: %+v", node.url, string(msg))
				data, err := http.Post(node.url, []byte(msg))
				if err != nil {
					atomic.AddInt64(&client.sendFailureTimes, int64(1))
					atomic.AddInt64(&node.sendFailureTimes, int64(1))
					atomic.AddInt32(&node.failureTimesFlag, int32(1))
					failure_times := atomic.LoadInt32(&node.failureTimesFlag)
					// 如果连续3次错误，标志位故障
					if failure_times >= 3 {
						//发生故障
						log.Warnf("http service url %s post error happened max then 3, will be offline %s", node.url)
						node.lock.Lock()
						//node.isDown = true
						node.status ^= online
						node.status |= offline
						node.lock.Unlock()
					}
					log.Warnf("http service node %s failure times：%d", node.url, node.sendFailureTimes)
					client.cacheInit(node)
					client.addCache(node, []byte(msg))
				} else {
					node.lock.Lock()
					if node.status & offline > 0 {
						node.status ^= offline
						node.status |= online
					}
					node.lock.Unlock()
					failure_times := atomic.LoadInt32(&node.failureTimesFlag)
					//恢复即时清零故障计数
					if failure_times > 0 {
						atomic.StoreInt32(&node.failureTimesFlag, 0)
					}
					//对失败的cache进行重发
					client.sendCache(node)
				}
				log.Debugf("http service post to %s return %s", node.url, string(data))
			} else {
				// 故障节点，缓存需要发送的数据
				// 这里就需要一个map[string][10000][]byte，最多缓存10000条
				// 保持最新的10000条
				client.addCache(node, []byte(msg))
			}
		case <-client.ctx.Ctx.Done():
			if len(node.sendQueue) <= 0 {
				log.Debugf("http服务clientSendService退出：%s", node.url)
				return
			}
		}
	}
}

func (client *HttpService) SendAll(data map[string] interface{}) bool {
	if client.status & serviceDisable > 0 {
		return false
	}
	client.lock.Lock()
	defer client.lock.Unlock()

	for _, cgroup := range client.groups {
		if len(cgroup.nodes) <= 0 {
			continue
		}
		// length, 2 bytes
		//tableLen := int(msg[0]) + int(msg[1]<<8)
		// content
		table := data["table"].(string)//string(msg[2 : tableLen+2])
		jsonData, _:= json.Marshal(data)
		// check if the table name matches the filter
		if len(cgroup.filter) > 0 {
			found := false
			for _, f := range cgroup.filter {
				match, err := regexp.MatchString(f, table)
				if err != nil {
					continue
				}
				if match {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		for _, cnode := range cgroup.nodes {
			log.Debugf("http send broadcast: %s=>%s", cnode.url, string(jsonData))
			if len(cnode.sendQueue) >= cap(cnode.sendQueue) {
				log.Warnf("http send buffer full(weight):%s, %s", cnode.url, string(jsonData))
				continue
			}
			cnode.sendQueue <- string(jsonData)
		}
	}

	return true
}

func (client *HttpService) Close() {
	log.Debug("http service closing, waiting for buffer send complete.")
	for _, cgroup := range client.groups {
		if len(cgroup.nodes) > 0 {
			client.wg.Wait()
			break
		}
	}
	log.Debug("http service closed.")
}

func (client *HttpService) Reload() {
	config, _ := getHttpConfig()
	log.Debug("http service reloading...")

	status := serviceDisable
	if config.Enable {
		status ^= serviceDisable
		status |= serviceEnable
	}


	client.status = status//config.Enable
	for name := range client.groups {
		delete(client.groups, name)
	}

	for _, cgroup := range config.Groups {
		group := &httpGroup{
			name: cgroup.Name,
		}
		group.filter = make([]string, len(cgroup.Filter))
		group.filter = append(group.filter[:0], cgroup.Filter...)

		nc := len(cgroup.Nodes)
		group.nodes = make([]*httpNode, nc)
		for i := 0; i < nc; i++ {
			group.nodes[i] = &httpNode{
				url:              cgroup.Nodes[i],
				sendQueue:        make(chan string, TCP_MAX_SEND_QUEUE),
				sendTimes:        int64(0),
				sendFailureTimes: int64(0),
				lock:             new(sync.Mutex),
				failureTimesFlag: int32(0),
				errorCheckTimes:  int64(0),
				status:           online | cacheNotReady | cacheNotFull,
			}
		}
		client.groups[cgroup.Name] = group
	}
	log.Debug("http service reloaded.")
}

func (client *HttpService) AgentStart(serviceIp string, port int) {

}
func (client *HttpService) AgentStop() {

}