package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/cxpsemea/Cx1ClientGo"
	"github.com/cxpsemea/cxql-harness/internal/harness"
	"github.com/cxpsemea/cxql-harness/internal/llm"
	"github.com/cxpsemea/cxqlmcp/mcp"
	"github.com/sirupsen/logrus"
	easy "github.com/t-tomalak/logrus-easy-formatter"
)

func main() {
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	myformatter := &easy.Formatter{}
	myformatter.TimestampFormat = "2006-01-02 15:04:05.000"
	myformatter.LogFormat = "[%lvl%][%time%] %msg%\n"
	logger.SetFormatter(myformatter)

	f, _ := os.Create("log.txt")
	defer f.Close()
	outwriter := io.MultiWriter(os.Stdout, f)
	logger.SetOutput(outwriter)

	backend := flag.String("backend", "ollama", "LLM backend: ollama, openai, anthropic")
	model := flag.String("model", "", "Model name (default depends on backend)")
	maxIter := flag.Int("max-iter", 20, "Maximum reasoning iterations")
	findingURL := flag.String("link", "", "Link to the finding to be addressed as a False Positive")
	prompt := flag.String("prompt", "", "Optional: Guidance to the LLM regarding the problem and/or desired solution")

	var cx1client *Cx1ClientGo.Cx1Client
	httpClient := &http.Client{}
	var llmClient llm.LLM
	var h *harness.Harness
	var err error

	fs := flag.NewFlagSet("testCheck", flag.ContinueOnError)
	test := fs.Bool("test", false, "Test mode")
	_ = fs.Parse(os.Args[1:])
	if !*test {
		if *findingURL == "" {
			fmt.Fprintf(os.Stderr, "A finding URL is required")
			flag.PrintDefaults()
			os.Exit(1)
		}

		cx1client, err = Cx1ClientGo.NewClient(httpClient, logger)
		if err != nil {
			logger.Fatalf("Error creating client: %s", err)
		}
		logger.Infof("Initialized client: %s", cx1client.String())

		switch *backend {
		case "ollama":
			m := "qwen2.5-coder:14b"
			if *model != "" {
				m = *model
			}
			llmClient, err = llm.NewOllama(m)
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

		h = harness.New(logger, mcp.NewMCP(cx1client, logger), llmClient, *maxIter, true)
	} else {
		h = harness.New(logger, harness.NewTestMCP(), llm.NewTestLLM(logger), *maxIter, true)
	}

	err = h.Run(context.Background(), *findingURL, *prompt)
	if err != nil {
		log.Fatalf("harness error: %v", err)
	}
	log.Println("Finished without error")
}
