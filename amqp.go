package metrics_amqp

import (
	"encoding/json"
	"github.com/rcrowley/go-metrics"
	"github.com/streadway/amqp"
	"log"
	uurl "net/url"
	"time"
)

type reporter struct {
	reg      metrics.Registry
	interval time.Duration

	url        string
	connection *amqp.Connection
	qname      string
}

type datum struct {
	Name   string
	Fields map[string]interface{}
}

func Amqp(r metrics.Registry, d time.Duration, url string, qn string) {
	_, err := uurl.Parse(url)

	if err != nil {
		log.Printf("Unable to parse AMQP URL %s: %s", url, err)
		return
	}

	rep := &reporter{
		reg:      r,
		interval: d,
		url:      url,
		qname:    qn,
	}

	if err := rep.makeClient(); err != nil {
		log.Printf("Unable to connect to AMQP URL %s: %v", url, err)
		return
	}
	log.Printf("AMQP connection to %s established", url)
	rep.run()
}

func (r *reporter) makeClient() (err error) {
	r.connection, err = amqp.Dial(r.url)
	if err != nil {
		return err
	}
	channel, err := r.connection.Channel()
	defer channel.Close()
	if err != nil {
		return err
	}

	log.Printf("Creating AMQP queue %s", r.qname)
	durable, exclusive := false, false
	autoDelete, noWait := true, true
	q, _ := channel.QueueDeclare(r.qname, durable, autoDelete, exclusive, noWait, nil)
	channel.QueueBind(q.Name, "#", "amq.topic", false, nil)

	return
}

func (r *reporter) run() {
	for t := range time.Tick(r.interval * time.Second) {
		if err := r.send(); err != nil {
			log.Printf("Unable to send metrics to AMQP %s at %s: %v", r.url, t, err)
		}
		log.Printf("Sent metrics")
	}
}

func (r *reporter) send() (err error) {
	var m datum
	r.reg.Each(func(name string, i interface{}) {
		now := time.Now()
		switch metric := i.(type) {
		case metrics.Counter:
			m = datum{
				Name:   name,
				Fields: map[string]interface{}{"count": metric.Count()},
			}
		case metrics.Gauge: //, metrics.GaugeFloat64:
			m = datum{
				Name:   name,
				Fields: map[string]interface{}{"value": metric.Value()},
			}
		case metrics.GaugeFloat64:
			m = datum{
				Name:   name,
				Fields: map[string]interface{}{"value": metric.Value()},
			}
		case metrics.Histogram:
			ps := metric.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999, 0.9999})
			// Remember to take a snapshot of complex metrics for consistent data
			metric = metric.Snapshot()
			m = datum{
				Name: name,
				Fields: map[string]interface{}{"count": metric.Count(),
					"max":      metric.Max(),
					"mean":     metric.Mean(),
					"min":      metric.Min(),
					"stddev":   metric.StdDev(),
					"variance": metric.Variance(),
					"p50":      ps[0],
					"p75":      ps[1],
					"p95":      ps[2],
					"p99":      ps[3],
					"p999":     ps[4],
					"p9999":    ps[5]},
			}
		case metrics.Meter:
			metric = metric.Snapshot()
			m = datum{
				Name: name,
				Fields: map[string]interface{}{"count": metric.Count(),
					"m1":   metric.Rate1(),
					"m5":   metric.Rate5(),
					"m15":  metric.Rate15(),
					"mean": metric.RateMean()},
			}
		case metrics.Timer:
			metric = metric.Snapshot()
			ps := metric.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999, 0.9999})
			m = datum{
				Name: name,
				Fields: map[string]interface{}{"count": metric.Count(),
					"max":      metric.Max(),
					"mean":     metric.Mean(),
					"min":      metric.Min(),
					"stddev":   metric.StdDev(),
					"variance": metric.Variance(),
					"p50":      ps[0],
					"p75":      ps[1],
					"p95":      ps[2],
					"p99":      ps[3],
					"p999":     ps[4],
					"p9999":    ps[5],
					"m1":       metric.Rate1(),
					"m5":       metric.Rate5(),
					"m15":      metric.Rate15(),
					"meanrate": metric.RateMean()},
			}
		}
		message, err := json.Marshal(m)
		if err != nil {
			log.Printf("Unable to marshal %s: %v", m, err)
			return
		}
		// log.Printf("Message: %v\tJSON: %v", m, message)

		channel, err := r.connection.Channel()
		if err != nil {
			log.Printf("Unable to establish AMQP connection %s: %v", r.url, err)
			return
		}

		msg := amqp.Publishing{
			DeliveryMode: 1,
			Timestamp:    now,
			ContentType:  "text/json",
			Body:         message,
		}
		mandatory, immediate := false, false
		channel.Publish("amq.topic", "ping", mandatory, immediate, msg)
	})
	return
}
