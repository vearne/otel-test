// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	zlog "github.com/vearne/otel-test/log"
	"github.com/vearne/otel-test/myotel"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	//stdout "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"github.com/go-resty/resty/v2"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/baggage"
	"golang.org/x/sync/errgroup"
)

var rdb *redis.Client

func init() {
	myotel.InitTracerProvider()
	myotel.InitMeterProvider()
	err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second))
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	zlog.InitLogger("/tmp/otel.log", "debug")
	// init redis
	// 初始化Redis
	rdb = redis.NewClient(&redis.Options{
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
	r.Use(otelgin.Middleware("my-server"))
	tmplName := "user"
	tmplStr := "user {{ .name }} (id {{ .id }})\n"
	tmpl := template.Must(template.New(tmplName).Parse(tmplStr))
	r.SetHTMLTemplate(tmpl)
	r.GET("/users/:id", func(c *gin.Context) {
		id := c.Param("id")

		zlog.InfoContext(c.Request.Context(), "userID", zap.String("id", id))

		name := getUser(c, id)
		otelgin.HTML(c, http.StatusOK, tmplName, gin.H{
			"name": name,
			"id":   id,
		})
	})
	r.GET("/ping", func(c *gin.Context) {
		ctx := c.Request.Context()
		g, _ := errgroup.WithContext(ctx)

		g.Go(func() error {
			val, err := rdb.Incr(ctx, "helloCounter2").Result()
			if err != nil {
				zlog.ErrorContext(ctx, "ping", zap.Int64("val", val), zap.Error(err))
			} else {
				zlog.InfoContext(ctx, "ping", zap.Int64("val", val))
			}
			return nil
		})
		g.Go(func() error {
			hsetRes, err := rdb.HSet(ctx, "xyz", "def", 0).Result()
			if err != nil {
				zlog.ErrorContext(ctx, "ping", zap.Int64("setRes", hsetRes), zap.Error(err))
			} else {
				zlog.InfoContext(ctx, "ping", zap.Int64("setRes", hsetRes))
			}
			return nil
		})
		g.Wait()
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	r.GET("/sayHelloHttp", func(c *gin.Context) {
		ctx := c.Request.Context()
		val, err := rdb.Incr(ctx, "helloHttpCounter").Result()
		if err != nil {
			zlog.ErrorContext(ctx, "test hello http", zap.Int64("val", val), zap.Error(err))
		} else {
			zlog.InfoContext(ctx, "test hello http", zap.Int64("val", val))
		}

		client := resty.NewWithClient(&http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		})
		resp, err := client.R().
			Get("http://localhost:18001/sayHello")

		c.JSON(http.StatusOK, gin.H{
			"message": resp.String(),
		})
	})
	r.GET("/sayHelloGrpc", func(c *gin.Context) {
		ctx := c.Request.Context()

		val, err := rdb.Incr(c.Request.Context(), "helloGrpcCounter").Result()
		if err != nil {
			zlog.ErrorContext(ctx, "test hello grpc", zap.Int64("val", val), zap.Error(err))
		} else {
			zlog.InfoContext(ctx, "test hello grpc", zap.Int64("val", val))
		}

		conn, err := grpc.Dial("localhost:50051",
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		)

		if err != nil {
			log.Fatalf("did not connect: %v", err)
		}
		defer conn.Close()
		client := pb.NewGreeterClient(conn)

		reqBaggage, err := baggage.New()
		if err != nil {
			zlog.ErrorContext(ctx, "use Baggage", zap.Error(err))
		} else {
			zlog.InfoContext(ctx, "use Baggage")
			member, _ := baggage.NewMember("mykey", "myvalue")
			reqBaggage, _ = reqBaggage.SetMember(member)
		}

		ctx = baggage.ContextWithBaggage(ctx, reqBaggage)
		// Contact the server and print out its response.
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		r, err := client.SayHello(ctx, &pb.HelloRequest{Name: "lily"})
		if err != nil {
			zlog.Error("could not greet", zap.Error(err))
			c.JSON(http.StatusOK, gin.H{
				"error": err.Error(),
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"message": r.GetMessage(),
			})
		}
	})
	_ = r.Run(":8080")
}

func getUser(c *gin.Context, id string) string {
	// Pass the built-in `context.Context` object from http.Request to OpenTelemetry APIs
	// where required. It is available from gin.Context.Request.Context()
	tracer := otel.Tracer("otel-test")
	_, span := tracer.Start(c.Request.Context(), "getUser", oteltrace.WithAttributes(attribute.String("id", id)))
	defer span.End()
	if id == "123" {
		return "otelgin tester"
	}
	return "unknown"
}
