module github.com/cxpsemea/cxql-harness

go 1.26.3

require (
	github.com/cxpsemea/Cx1ClientGo v0.1.67
	github.com/cxpsemea/cxqlmcp/mcp v0.0.0
	github.com/sirupsen/logrus v1.9.4
	github.com/t-tomalak/logrus-easy-formatter v0.0.0-20190827215021-c074f06c5816
)

require (
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/modelcontextprotocol/go-sdk v1.6.1 // indirect
	github.com/segmentio/asm v1.2.1 // indirect
	github.com/segmentio/encoding v0.5.4 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/exp v0.0.0-20260718201538-764159d718ef // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

replace (
	github.com/cxpsemea/Cx1ClientGo v0.1.59 => ../Cx1ClientGo
	github.com/cxpsemea/cxqlmcp/mcp v0.0.0 => ../cxqlmcp/mcp
)
