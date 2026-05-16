package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Options configures a gRPC runner.
type Options struct {
	Target      string
	Timeout     time.Duration
	TLS         bool
	ServiceName string
	MethodName  string
	TokenField  string
}

// FullMethod returns the fully qualified gRPC method path.
func (o Options) FullMethod() string {
	if o.ServiceName == "" || o.MethodName == "" {
		return ""
	}
	return fmt.Sprintf("/%s/%s", o.ServiceName, o.MethodName)
}

// methodDescriptor holds the resolved input/output descriptors for a method.
type methodDescriptor struct {
	Input  protoreflect.MessageDescriptor
	Output protoreflect.MessageDescriptor
}

func dial(target string, tlsEnabled bool) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if tlsEnabled {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	return grpc.NewClient(target, opts...)
}

// resolveMethod uses server reflection to look up method descriptors.
func resolveMethod(ctx context.Context, conn *grpc.ClientConn, service, method string) (*methodDescriptor, error) {
	refClient := grpc_reflection_v1.NewServerReflectionClient(conn)
	stream, err := refClient.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("reflection stream: %w", err)
	}

	symbol := fmt.Sprintf("%s.%s", service, method)
	if err := stream.Send(&grpc_reflection_v1.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: symbol,
		},
	}); err != nil {
		return nil, fmt.Errorf("send reflection request: %w", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("recv reflection response: %w", err)
	}
	_ = stream.CloseSend()

	fdResp := resp.GetFileDescriptorResponse()
	if fdResp == nil {
		return nil, fmt.Errorf("unexpected reflection response for %s: %v", symbol, resp)
	}

	fds := &descriptorpb.FileDescriptorSet{}
	for _, raw := range fdResp.FileDescriptorProto {
		var fd descriptorpb.FileDescriptorProto
		if err := proto.Unmarshal(raw, &fd); err != nil {
			return nil, fmt.Errorf("unmarshal file descriptor: %w", err)
		}
		fds.File = append(fds.File, &fd)
	}

	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, fmt.Errorf("build file registry: %w", err)
	}

	desc, err := files.FindDescriptorByName(protoreflect.FullName(service))
	if err != nil {
		return nil, fmt.Errorf("find service %s: %w", service, err)
	}
	svcDesc, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%s is not a service descriptor", service)
	}

	methodDesc := svcDesc.Methods().ByName(protoreflect.Name(method))
	if methodDesc == nil {
		return nil, fmt.Errorf("method %s not found in service %s", method, service)
	}

	return &methodDescriptor{
		Input:  methodDesc.Input(),
		Output: methodDesc.Output(),
	}, nil
}

// buildInput converts a JSON body to a dynamic proto message using the input descriptor.
func buildInput(body []byte, desc protoreflect.MessageDescriptor) (proto.Message, error) {
	msg := dynamicpb.NewMessage(desc)
	if len(body) == 0 {
		return msg, nil
	}
	if err := protojson.Unmarshal(body, msg); err != nil {
		return nil, fmt.Errorf("unmarshal json body to proto: %w", err)
	}
	return msg, nil
}

// toJSON marshals a proto message to JSON bytes for token extraction.
func toJSON(msg proto.Message) []byte {
	data, err := protojson.Marshal(msg)
	if err != nil {
		return nil
	}
	return data
}

// extractTokenCount attempts to find a token count in JSON.
func extractTokenCount(data []byte, tokenField string) int64 {
	if tokenField == "" || len(data) == 0 {
		return 0
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return 0
	}
	return findInt(payload, strings.ToLower(tokenField))
}

func findInt(node any, key string) int64 {
	switch v := node.(type) {
	case map[string]any:
		for k, val := range v {
			if strings.ToLower(k) == key {
				switch n := val.(type) {
				case float64:
					return int64(n)
				case int64:
					return n
				}
			}
			if nested := findInt(val, key); nested != 0 {
				return nested
			}
		}
	}
	return 0
}
