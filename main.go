package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/cxpsemea/Cx1ClientGo"
	"github.com/cxpsemea/cxql-harness/internal/harness"
	"github.com/cxpsemea/cxql-harness/internal/llm"
	"github.com/cxpsemea/cxql-harness/internal/logging"
	"github.com/cxpsemea/cxqlmcp/mcp"
	"github.com/sirupsen/logrus"
	easy "github.com/t-tomalak/logrus-easy-formatter"
)

func main() {
	myformatter := &easy.Formatter{}
	myformatter.TimestampFormat = "2006-01-02 15:04:05.000"
	myformatter.LogFormat = "[%lvl%][%time%] %msg%\n"

	logger, closeLog, err := logging.New("log.txt", logrus.InfoLevel, myformatter)
	if err != nil {
		log.Fatalf("failed to open log.txt: %v", err)
	}
	defer closeLog()

	backend := flag.String("backend", "ollama", "LLM backend: ollama, openai, anthropic")
	model := flag.String("model", "", "Model name (default depends on backend)")
	address := flag.String("address", "http://localhost:11434", "LLM server address")
	maxIter := flag.Int("max-iter", 20, "Maximum reasoning iterations")
	prompt := flag.String("prompt", "", "Optional: Guidance to the LLM regarding the problem and/or desired solution")
	config := flag.String("config", "conf.json", "Configuration file containing target FP, TPList, and TNList - see example-conf.json")

	var cx1client *Cx1ClientGo.Cx1Client
	httpClient := &http.Client{}
	var llmClient llm.LLM
	var h *harness.Harness

	test := false
	if !test {
		if *config == "" {
			fmt.Fprintf(os.Stderr, "A configuration file is required")
			flag.PrintDefaults()
			os.Exit(1)
		}

		cx1client, err = Cx1ClientGo.NewClient(httpClient, logger)
		if err != nil {
			logger.Fatalf("Error creating client: %s", err)
		}
		logger.Infof("Initialized client: %s", cx1client.String())

		cx1client.SetDeprecationWarning(false)

		switch *backend {
		case "ollama":
			m := "qwen3-coder:30b"
			if *model != "" {
				m = *model
			}
			llmClient, err = llm.NewOllama(m, *address)
		case "openai":
			m := "gpt-4o"
			if *model != "" {
				m = *model
			}
			llmClient, err = llm.NewOpenAI(m)
		case "anthropic":
			m := "claude-sonnet-4-5"
			if *model != "" {
				m = *model
			}
			llmClient, err = llm.NewAnthropic(m)
		default:
			log.Fatalf("unknown backend: %s", *backend)
		}
		if err != nil {
			log.Fatalf("failed to create LLM client: %v", err)
		}

		h = harness.New(logger, mcp.NewMCP(cx1client, logger), llmClient, *maxIter, false)
	} else {
		h = harness.New(logger, harness.NewTestMCP(), llm.NewTestLLM(logger), *maxIter, false)
	}

	var conf struct {
		Finding string
		TPList  []string
		TNList  []string
	}

	data, err := os.ReadFile(*config)
	if err != nil {
		log.Fatalf("Failed to read config file %s: %s", *config, err)
	}

	err = json.Unmarshal(data, &conf)
	if err != nil {
		log.Fatalf("Failed to parse config file %s: %s", *config, err)
	}

	err = h.Run(context.Background(), conf.Finding, *prompt, conf.TPList, conf.TNList)
	if err != nil {
		log.Fatalf("harness error: %v", err)
	}
	log.Println("Finished without error")
}
