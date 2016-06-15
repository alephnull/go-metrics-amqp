# go-metrics-amqp
A reporter for go-metrics that uses a message bus as the transport. This has been tested with [cloudamqp](https://www.cloudamqp.com/docs/go.html).

The implementation is based on the InfluxDB implementation at [vrischmann/go-metrics-influxdb](https://github.com/vrischmann/go-metrics-influxdb).

The metrics are serialised by hand and then marshalled as JSON objects before being posted to a queue. There are no assumptions on the type of queue or any queue parameter and so you are free to implement the queue in any form you need.

Please report issues on the issue tracker on github.

# Usage

    import "github.com/alephnull/go-metrics-amqp"

	go metrics_amqp.Amqp(metrics.DefaultRegistry, 5, "<AMQP_URL>", "<QUEUE_NAME>")
    
## Example

    package main

    import(
        "github.com/rcrowley/go-metrics"
        "github.com/alephnull/go-metrics-amqp"
        "github.com/streadway/amqp"
        "time"
        "log"
    )

    // This is a function to retrieve the messages for the beginnings
    // of a test suite
    func consume(url string, qname string) {
            connection, _ := amqp.Dial(url)
            defer connection.Close()
        channel, _ := connection.Channel()
        defer channel.Close()
        durable, exclusive := false, false
        autoDelete, noWait := true, true
        q, _ := channel.QueueDeclare(qname, durable, autoDelete, exclusive, noWait, nil)
        channel.QueueBind(q.Name, "#", "amq.topic", false, nil)
        autoAck, exclusive, noLocal, noWait := false, false, false, false
        messages, _ := channel.Consume(q.Name, "", autoAck, exclusive, noLocal, noWait, nil)
        multiAck := false
        for msg := range messages {
            log.Println("Body:", string(msg.Body), "Timestamp:", msg.Timestamp)
            msg.Ack(multiAck)
        }
    }

    func main() {
        go metrics_amqp.Amqp(metrics.DefaultRegistry, 5, "<AMQP_URL>i", "<QUEUE_NAME>")

        // This is for testing
        go consume("<AMQP_URL>i", "<QUEUE_NAME>")

        // go metrics.Log(metrics.DefaultRegistry, 5 * time.Second, log.New(os.Stderr, "metrics: ", log.Lmicroseconds))

        c := metrics.NewCounter()
        metrics.Register("c0", c)
        c.Inc(42)

        g := metrics.NewGauge()
        metrics.Register("g0", g)
        g.Update(42)

        gf := metrics.NewGaugeFloat64()
        metrics.Register("g1", gf)
        gf.Update(42.625)

        m := metrics.NewMeter()
        metrics.Register("m0", m)
        m.Mark(43)

        t := metrics.NewTimer()
        metrics.Register("t0", t)
        t.Time(func() { time.Sleep(2) })

        s := metrics.NewUniformSample(10)
        h := metrics.NewHistogram(s)
        metrics.Register("h0", h)
        h.Update(47)

        log.Printf("Time: %s", time.Now())
        time.Sleep(20 * time.Second)
        log.Printf("Time: %s", time.Now())
    }

