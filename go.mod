module github.com/cxpsemea/cxql-harness

go 1.26.3

require (
	github.com/cxpsemea/Cx1ClientGo v0.1.59
	github.com/cxpsemea/cxqlmcp/mcp v0.0.0
	github.com/sirupsen/logrus v1.9.4
	github.com/t-tomalak/logrus-easy-formatter v0.0.0-20190827215021-c074f06c5816
)

require (
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/modelcontextprotocol/go-sdk v1.6.1 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.5.4 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/exp v0.0.0-20260611194520-c48552f49976 // indirect
	golang.org/x/oauth2 v0.35.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
)

replace (
	github.com/cxpsemea/Cx1ClientGo v0.1.59 => ../Cx1ClientGo
	github.com/cxpsemea/cxqlmcp/mcp v0.0.0 => ../cxqlmcp/mcp
)
