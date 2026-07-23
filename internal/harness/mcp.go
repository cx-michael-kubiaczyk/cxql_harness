package harness

type mcpi interface {
	CreateSessionFromURL(path string) string

	// returns the current state:
	//   - current finding details (description, recommendation),
	//   - code snippets with the dataflow path
	//   - the CxQL query that was used to find the issue
	GetCurrentState() string

	// return the explanation of the finding eg: Missing_HSTS description + recommendation
	GetFindingDetails() string

	// returns the source code involved in the finding or query dataflow
	GetCodeSnippets() string

	// returns the high-level description of the query process
	GetHLD() string

	// returns a list of files + lines matching the search string, eg /src/somefile.java:123 this is the line of code
	SearchCode(substring string) string

	// returns the source code for a specific file along with any comments added by the MCP process (dataflow markers)
	ShowSourceCode(path string, lineStart, lineEnd int) string

	// returns the CxQL hierarchy + source code for a given query, eg: Missing_HSTS_Header
	GetQueryInfo(language, group, name string) string

	// checks if the original finding is found in the audit session or not
	CheckOriginalFinding() string

	// runs an existing query and returns the results (which may be multiple dataflow paths)
	RunQuery(language, group, query string) string

	// runs an updated version of a CxQL query, without saving the changes, and returns the results (which may be multiple dataflow paths)
	TestQuery(language, group, query, code string) string

	// saves an updated version of a CxQL query based on the last successful RunQuery call.
	SaveQuery(language, group, query string) string
}
