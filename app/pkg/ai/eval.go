package ai

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"connectrpc.com/connect"
	"github.com/go-logr/zapr"
	"github.com/jlewi/cloud-assistant/app/pkg/config"
	"github.com/jlewi/cloud-assistant/app/pkg/logs"
	"github.com/jlewi/cloud-assistant/protos/gen/cassie"
	"github.com/jlewi/cloud-assistant/protos/gen/cassie/cassieconnect"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

type Asserter interface {
	Assert(ctx context.Context, as *cassie.Assertion, blocks map[string]*cassie.Block) error
}

type shellRequiredFlag struct{}

func (s shellRequiredFlag) Assert(ctx context.Context, as *cassie.Assertion, blocks map[string]*cassie.Block) error {
	shellFlag := as.GetShellRequiredFlag()
	command := shellFlag.Command
	flags := shellFlag.Flags
	contain_command := false                    // Tracks if the target command is found in any code block
	as.Result = cassie.Assertion_RESULT_SKIPPED // Default result is SKIPPED unless the command is found
	for _, block := range blocks {
		if block.Kind == cassie.BlockKind_CODE {
			if strings.Contains(block.Contents, command) { // Check if the code block contains the target command
				if !contain_command {
					contain_command = true
					as.Result = cassie.Assertion_RESULT_TRUE // Set to PASSED if the command is present (may be overridden below)
				}
				for _, flag := range flags { // If the command is present, check for all required flags
					if !strings.Contains(block.Contents, flag) {
						as.Result = cassie.Assertion_RESULT_FALSE // Set to FAILED if any required flag is missing
					}
				}
			}
		}
	}
	fmt.Println("shellRequiredFlag", as.Name, as.Result)
	return nil
}

type toolInvocation struct{}

func (t toolInvocation) Assert(ctx context.Context, as *cassie.Assertion, blocks map[string]*cassie.Block) error {
	// TODO: implement
	fmt.Println("toolInvocation", as.Name)
	return nil
}

type fileRetrieved struct{}

func (f fileRetrieved) Assert(ctx context.Context, as *cassie.Assertion, blocks map[string]*cassie.Block) error {
	// TODO: implement
	fmt.Println("fileRetrieved", as.Name)
	return nil
}

type llmJudge struct{}

func (l llmJudge) Assert(ctx context.Context, as *cassie.Assertion, blocks map[string]*cassie.Block) error {
	// TODO: implement
	fmt.Println("llmJudge", as.Name)
	return nil
}

var registry = map[cassie.Assertion_Type]Asserter{
	cassie.Assertion_TYPE_SHELL_REQUIRED_FLAG: shellRequiredFlag{},
	cassie.Assertion_TYPE_TOOL_INVOKED:        toolInvocation{},
	cassie.Assertion_TYPE_FILE_RETRIEVED:      fileRetrieved{},
	cassie.Assertion_TYPE_LLM_JUDGE:           llmJudge{},
}

func EvalFromProto(protoFile string, cfg *config.CloudAssistantConfig) (map[string]*cassie.Block, error) {
	data, err := os.ReadFile(protoFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read proto file %q", protoFile)
	}
	var dataset cassie.EvalDataset
	if err := proto.Unmarshal(data, &dataset); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal proto file %q", protoFile)
	}
	blocks, err := runInference(dataset.Samples[0].InputText, cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to run inference")
	}
	for _, sample := range dataset.Samples {
		for _, assertion := range sample.Assertions {
			err := registry[assertion.Type].Assert(context.TODO(), assertion, blocks)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to assert %q", assertion.Name)
			}
		}
	}
	return blocks, nil
}

func runInference(input string, cfg *config.CloudAssistantConfig) (map[string]*cassie.Block, error) {
	log := zapr.NewLoggerWithOptions(zap.L(), zapr.AllowZapFields(true))

	blocks := make(map[string]*cassie.Block)

	Block := cassie.Block{
		Kind:     cassie.BlockKind_MARKUP,
		Contents: "This is a block",
	}

	log.Info("Block", logs.ZapProto("block", &Block))

	baseURL := cfg.TargetURL
	if baseURL == "" {
		return blocks, errors.New("TargetURL is not set in config")
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		log.Error(err, "Failed to parse URL")
		return blocks, errors.Wrapf(err, "Failed to parse URL")
	}

	var client cassieconnect.BlocksServiceClient

	var options []connect.ClientOption
	if u.Scheme == "https" {
		// Configure the TLS settings
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true, // Set to true only for testing; otherwise validate the server's certificate
		}

		client = cassieconnect.NewBlocksServiceClient(
			&http.Client{
				Transport: &http2.Transport{
					TLSClientConfig: tlsConfig,
					DialTLSContext: func(ctx context.Context, network, addr string, config *tls.Config) (net.Conn, error) {
						// Create a secure connection with TLS
						return tls.Dial(network, addr, config)
					},
				},
			},
			baseURL,
			options...,
		)
	} else {
		client = cassieconnect.NewBlocksServiceClient(
			&http.Client{
				Transport: &http2.Transport{
					AllowHTTP: true,
					DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
						// Use the standard Dial function to create a plain TCP connection
						return net.Dial(network, u.Host)
					},
				},
			},
			baseURL,
			options...,
		)
	}

	ctx := context.Background()
	genReq := &cassie.GenerateRequest{
		Blocks: []*cassie.Block{
			{
				Kind:     cassie.BlockKind_MARKUP,
				Role:     cassie.BlockRole_BLOCK_ROLE_USER,
				Contents: input,
			},
		},
	}
	req := connect.NewRequest(genReq)
	cookie := &http.Cookie{
		Name:  "cassie-session",
		Value: cfg.CassieCookie, // supply the real value here, temporary solution
		Path:  "/",              // adjust if needed
	}
	req.Header().Add("Cookie", cookie.String())
	stream, err := client.Generate(ctx, req)
	if err != nil {
		return blocks, errors.Wrapf(err, "Failed to create generate stream")
	}

	// Receive responses
	for stream.Receive() {
		response := stream.Msg()

		for _, block := range response.Blocks {
			blocks[block.Id] = block

			options := protojson.MarshalOptions{
				Multiline: true,
				Indent:    "  ", // Two spaces for indentation
			}

			// Marshal the protobuf message to JSON
			jsonData, err := options.Marshal(block)
			if err != nil {
				log.Error(err, "Failed to marshal block to JSON")
			} else {
				log.Info("Block", "block", string(jsonData))
			}
		}

	}

	if stream.Err() != nil {
		return blocks, errors.Wrapf(stream.Err(), "Error receiving response")
	}
	return blocks, nil
}

// EvalFromYAML reads a YAML file, converts it to JSON, and unmarshals it into cassie.EvalDataset using protojson.
func EvalFromYAML(yamlFile string, cfg *config.CloudAssistantConfig) (map[string]*cassie.Block, error) {
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read yaml file %q", yamlFile)
	}
	// Unmarshal YAML to generic map
	var yamlObj interface{}
	if err := yaml.Unmarshal(data, &yamlObj); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal yaml file %q", yamlFile)
	}
	// Convert YAML to JSON
	jsonData, err := json.Marshal(yamlObj)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal yaml to json for file %q", yamlFile)
	}
	var dataset cassie.EvalDataset
	if err := protojson.Unmarshal(jsonData, &dataset); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal json to proto for file %q", yamlFile)
	}
	blocks, err := runInference(dataset.Samples[0].InputText, cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to run inference")
	}
	for _, sample := range dataset.Samples {
		for _, assertion := range sample.Assertions {
			err := registry[assertion.Type].Assert(context.TODO(), assertion, blocks)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to assert %q", assertion.Name)
			}
		}
	}
	return blocks, nil
}
