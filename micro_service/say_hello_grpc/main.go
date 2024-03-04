package main

import (
	"fmt"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	zlog "github.com/vearne/otel-test/log"
	"github.com/vearne/otel-test/myotel"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"sync/atomic"
	"time"

	_ "github.com/apache/skywalking-go"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip" // 会完成gzip Compressor的注册
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/reflection"
)

func init() {
	myotel.InitTracerProvider()
	myotel.InitMeterProvider()
	err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second))
	if err != nil {
		log.Fatal(err)
	}
}

var counter uint64 = 0

const (
	port = ":50051"
)

var rdb *redis.Client

func main() {
	zlog.InitLogger("/tmp/sayHelloGrpc.log", "debug")

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

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	pb.RegisterGreeterServer(s, &server{})
	// Register reflection service on gRPC server.
	reflection.Register(s)

	log.Println("say_hello_grpc starting...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

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
	val, err := rdb.Incr(ctx, "svc-sayHello-grpc").Result()
	if err != nil {
		zlog.ErrorContext(ctx, "test hello", zap.Int64("val", val), zap.Error(err))
	} else {
		zlog.InfoContext(ctx, "test hello", zap.Int64("val", val))
	}

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
