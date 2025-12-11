package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	zlog "github.com/vearne/otel-test/log"
	"github.com/vearne/otel-test/micro_service/microtel"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.uber.org/zap"
)

func init() {
	microtel.InitTracerProvider()
	microtel.InitMeterProvider()
	err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second))
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	zlog.InitLogger("/tmp/sayHelloHttp.log", "debug")

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Enable tracing instrumentation.
	if err := redisotel.InstrumentTracing(rdb); err != nil {
		panic(err)
	}

	// Enable metrics instrumentation.
	if err := redisotel.InstrumentMetrics(rdb); err != nil {
		panic(err)
	}

	r := gin.New()
	r.Use(otelgin.Middleware("say-hello-http"))
	r.GET("/sayHello", func(c *gin.Context) {
		// print Headers
		for key, val := range c.Request.Header {
			fmt.Printf("%v:%v\n", key, val)
		}
		ctx := c.Request.Context()
		val, err := rdb.Incr(ctx, "svc-sayHello-http").Result()
		if err != nil {
			zlog.ErrorContext(ctx, "test hello", zap.Int64("val", val), zap.Error(err))
		} else {
			zlog.InfoContext(ctx, "test hello", zap.Int64("val", val))
		}

		c.JSON(http.StatusOK, gin.H{
			"message": val,
		})
	})
	_ = r.Run(":18001")
}
