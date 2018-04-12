package kafka

import (
	"library/services"
	log "github.com/sirupsen/logrus"
	"github.com/Shopify/sarama"
	"sync"
)

const (
	isClose = 1 << iota
)
type Producer struct {
	services.Service
	AccessLogProducer sarama.AsyncProducer
	enable bool
	topic string
	filter []string
	status int
	lock *sync.Mutex
}

var _ services.Service = &Producer{}

func NewProducer() services.Service {
	config, _ := getConfig()
	if !config.Enable {
		return &Producer{
			enable:false,
		}
	}
	//brokers := strings.Split(",", config.Brokers)
	//[]string{"127.0.0.1:9092"}),
	log.Debugf("kafka config: %+v", *config)
	return &Producer{
		AccessLogProducer:newAccessLogProducer(config.Brokers),
		enable:true,
		topic:config.Topic,
		filter:config.Filter,
		status:0,
		lock:new(sync.Mutex),
	}
}

func (r *Producer) SendAll(table string, data []byte) bool {
	if !r.enable {
		return false
	}

	r.lock.Lock()
	if r.status & isClose > 0 {
		r.lock.Unlock()
		return false
	}
	r.lock.Unlock()

	entry := &accessLogEntry{
		Data:data,
	}

	if !services.MatchFilters(r.filter, table) {
		return false
	}
	log.Debugf("push to kafka: %v", data)
	// We will use the client's IP address as key. This will cause
	// all the access log entries of the same IP address to end up
	// on the same partition.
	r.AccessLogProducer.Input() <- &sarama.ProducerMessage{
		Topic: r.topic,
		Key:   sarama.StringEncoder(table),
		Value: entry,
	}
	return true
}
func (r *Producer) Start() {}
func (r *Producer) Close() {
	if !r.enable {
		return
	}
	r.lock.Lock()
	if r.status & isClose <= 0 {
		r.status |= isClose
	}
	r.lock.Unlock()
	if err := r.AccessLogProducer.Close(); err != nil {
		log.Println("Failed to shut down access log producer cleanly", err)
	}
}
func (r *Producer) Reload() {
	config, _ := getConfig()
	if r.AccessLogProducer != nil {
		r.AccessLogProducer.Close()
	}
	r.AccessLogProducer = newAccessLogProducer(config.Brokers)
	r.enable = true
	r.topic = config.Topic
	r.filter = config.Filter
}
func (r *Producer) Name() string {
	return "kafka"
}
func (r *Producer) SendRaw(data []byte) bool { return true }


