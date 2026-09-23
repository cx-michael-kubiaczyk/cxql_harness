package harness

import "fmt"

type TestMCP struct {
	runReqCount  int
	testReqCount int
	checkCount   int
}

func NewTestMCP() mcpi {
	return &TestMCP{}
}

func (m *TestMCP) CreateSessionFromURL(path string, TPList, TNList []string) string {
	return "The session was created successfully and the finding is present."
}
func (m *TestMCP) GetCurrentState() string {
	return ""
}
func (m *TestMCP) GetCurrentApplicationID() string {
	return "123"
}

func (m *TestMCP) GetHLD() string {
	return `When a CxSAST scan runs, various "CxQL queries" (written as C# code modules) are run against an AST (abstract syntax tree) representation of a codebase.
Each query returns a list of items representing nodes or dataflow paths through the AST.
The CxSAST product includes a variety of queries covering a range of security vulnerabilities, such as Reflected XSS or SQL Injection.
Queries that represent security vulnerabilities can call other queries to assemble the dataflows from the 'source node' to the 'sink node' in the AST.
Most queries can be 'overridden' allowing users to change the behavior of a specific query for a specific Project (a codebase), for an Application (a collection of Projects), or across the entire Tenant in the platform.
Query execution can also be chained, so a Tenant-wide override of Reflected_XSS can call the product default version via: result = base.Reflected_XSS();
Chained execution can go across levels, so a Project-level override can call an Application-level override which can call the Tenant-level override which can call the product default version.
When addressing false-positive results in a finding, the process follows these steps:
1. Examine the source code involved in the dataflow for the false-positive result.
2. Examine the target query generating the false-positive result to see the query source code, any existing overrides, and any other queries that the target query calls.
3. Examine any other queries and their overrides if they are part of the target query's call chains.
4. Run any queries involved in the target query's call chain to identify points of improvement.
5. Update existing overrides, or create new overrides (preferring Project-level overrides first, then Application, then Tenant) to improve the results and address the original false positive result.
6. Test the updated queries to evaluate the result.
7. Repeat the process as needed until the false positive is removed.`
}

func (m *TestMCP) GetCurrentProjectID() string {
	return "123"
}

func (m *TestMCP) GetQueryCode(level, lang, group, name string) string {
	return "some code"
}
func (m *TestMCP) RestoreQuery(level, language, group, query string) string {
	return fmt.Sprintf("Query %s.%s.%s restored to its original version.", language, group, query)
}
func (m *TestMCP) ConfigureCustomPreset(presetName string) string {
	return "Created custom preset with query xxx"
}

// return the explanation of the finding eg: Missing_HSTS description + recommendation
func (m *TestMCP) GetFindingDetails() string {
	return `Finding javascript.JavaScript_Medium_Threat.Missing_HSTS_Header details:
Description: 
The web-application does not define an HSTS header, leaving it vulnerable to attack.

Risk: 
Failure to set an HSTS header and provide it with a reasonable "max-age" value of at least one year may leave users vulnerable to Man-in-the-Middle attacks.

Recommendation: 
*   Before setting the HSTS header - consider the implications it may have:
    *   Forcing HTTPS will prevent any future use of HTTP, which could hinder some testing
    *   Disabling HSTS is not trivial, as once it is disabled on the site, it must also be disabled on the browser
*   Set the HSTS header either explicitly within application code, or using web-server configurations.
*   Ensure the "max-age" value for HSTS headers is set to 31536000 to ensure HSTS is strictly enforced for at least one year.
*   Include the "includeSubDomains" to maximize HSTS coverage, and ensure HSTS is enforced on all sub-domains under the current domain
    *   Note that this may prevent secure browser access to any sub-domains that utilize HTTP; however, use of HTTP is very severe and highly discouraged, even for websites that do not contain any sensitive information, as their contents can still be tampered via Man-in-the-Middle attacks to phish users under the HTTP domain.
*   Once HSTS has been enforced, submit the web-application's address to an HSTS preload list - this will ensure that, even if a client is accessing the web-application for the first time (implying HSTS has not yet been set by the web-application), a browser that respects the HSTS preload list would still treat the web-application as if it had already issued an HSTS header. Note that this requires the server to have a trusted SSL certificate, and issue an HSTS header with a maxAge of 1 year (31536000)
*   Note that this query is designed to return one result per application. This means that if more than one vulnerable response without an HSTS header is identified, only the first identified instance of this issue will be highlighted as a result. If a misconfigured instance of HSTS is identified (has a short lifespan, or is missing the "includeSubDomains" flag), that result will be flagged. Since HSTS is required to be enforced across the entire application to be considered a secure deployment of HSTS functionality, fixing this issue only where the query highlights this result is likely to produce subsequent results in other sections of the application; therefore, when adding this header via code, ensure it is uniformly deployed across the entire application. If this header is added via configuration, ensure that this configuration applies to the entire application.
*   Note that misconfigured HSTS headers that do not contain the recommended max-age value of at least one year or the "includeSubDomains" flag will still return a result for a missing HSTS header.`
}

// returns the source code involved in the finding or query dataflow
func (m *TestMCP) GetCodeSnippets() string {
	return `const cds = require('@sap/cds');
const helmet = require('helmet');
 
cds.on('bootstrap', async (app) => {
    app.use(
        helmet({
            strictTransportSecurity: {
                maxAge: 31536000,
                includeSubDomains: true,
                preload: true
            },
        })
    );
 
    app.head('/csp-probe', (req, res) => {
        res.setHeader('Cache-Control', 'no-store');
        return res.status(204).end(); // Finding Missing_HSTS_Header: step 0
    });
});`
}

// returns a list of files + lines matching the search string, eg /src/somefile.java:123 this is the line of code
func (m *TestMCP) SearchCode(substring string) string {
	return ""
}

// returns the source code for a specific file along with any comments added by the MCP process (dataflow markers)
func (m *TestMCP) ShowSourceCode(path string, lineStart, lineEnd int) string {
	return ""
}

// returns the CxQL hierarchy + source code for a given query, eg: Missing_HSTS_Header
func (m *TestMCP) GetQueryInfoFiltered(language, group, name string, view, edit []bool) string {
	return `[QUERY INFO]
The query JavaScript - JavaScript_Medium_Threat - Missing_HSTS_Header is included in the product

[PRODUCT DEFAULT QUERY INFO]
Source code: 
` + "```" + `csharp
result = Common_Medium_Threat.Missing_HSTS_Header().SanitizeCxList(Find_HSTS_Sanitize());
` + "```" + `
The following queries are called by this query:
 - JavaScript.General.Find_HSTS_Sanitize

[TENANT-LEVEL QUERY INFO]
Doesn't exist, cannot create (out of scope)

[APPLICATION-LEVEL QUERY INFO]
Can create`
}

// checks if the original finding is found in the audit session or not
func (m *TestMCP) CheckOriginalFinding() string {
	m.checkCount++
	if m.checkCount >= 3 {
		return "Finding is not present"
	}
	return "Finding is present"
}

func (m *TestMCP) CheckControlProjects() string {
	return "No regressions"
}

// runs an existing query and returns the results (which may be multiple dataflow paths)
func (m *TestMCP) RunQuery(level, language, group, query string) string {
	m.runReqCount++
	switch m.runReqCount {
	case 1:
		return "There were no results returned."
	default:
		return fmt.Sprintf("RunQuery %d: huh", m.runReqCount)
	}
}

// runs an updated version of a CxQL query, without saving the changes, and returns the results (which may be multiple dataflow paths)
func (m *TestMCP) TestQuery(level, language, group, query, code string) string {
	m.testReqCount++
	switch m.testReqCount {
	case 1:
		return `Error: The query JavaScript.General.Find_HSTS_Sanitize ran with errors, shown inline in the code below.
result = base.Find_(); // audit: Error: 'DynamicQuery_Runner.JavaScript.Corp.General' does not contain a definition for 'Find_'`
	case 2:
		return "There were no results returned."
	default:
		return fmt.Sprintf("RunQuery request %d undefined", m.testReqCount)
	}
}

// saves an updated version of a CxQL query based on the last successful RunQuery call.
func (m *TestMCP) SaveQuery(level, language, group, query, code string) string {
	return ""
}
