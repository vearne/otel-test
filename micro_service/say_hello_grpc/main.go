package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	zlog "github.com/vearne/zaplog"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"sync/atomic"

	_ "github.com/apache/skywalking-go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip" // 会完成gzip Compressor的注册
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/reflection"
)

var counter uint64 = 0

const (
	port = ":50051"
)

var rdb *redis.Client

type server struct {
	pb.UnimplementedGreeterServer
}

// SayHello implements helloworld.GreeterServer
func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	fmt.Println("pb.HelloRequest", in.Name)
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		fmt.Printf("get metadata error")
	}
	for key, val := range md {
		fmt.Printf("%v:%v\n", key, val)
	}
	val, err := rdb.Incr(context.Background(), "svc-sayHello-grpc").Result()
	zlog.Info("test hello", zap.Int64("val", val), zap.Error(err))

	atomic.AddUint64(&counter, 1)
	x := atomic.LoadUint64(&counter) % 3
	switch x {
	case 0:
		return &pb.HelloReply{Message: "Hello " + in.Name}, nil
	case 1:
		return &pb.HelloReply{Message: "Hello " + in.Name},
			status.Error(codes.DataLoss, "--DataLoss--")
	default:
		return &pb.HelloReply{Message: "Hello " + in.Name},
			status.Error(codes.Unauthenticated, "--Unauthenticated--")
	}
}

func main() {
	zlog.InitLogger("/tmp/sayHelloGrpc.log", "debug")

	// 添加Prometheus的相关监控
	// /metrics
	go func() {
		//time.Sleep(3 * time.Minute)
		r := gin.Default()
		r.GET("/metrics", gin.WrapH(promhttp.Handler()))
		r.Run(":9091")
	}()

	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterGreeterServer(s, &server{})
	// Register reflection service on gRPC server.
	reflection.Register(s)

	log.Println("say_hello_grpc starting...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
