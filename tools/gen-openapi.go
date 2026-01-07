package main

import (
	"fmt"
	"os"
	"sort"
	"text/template"
)

type Endpoint struct {
	Name        string
	Method      string
	Description string
	Parameters  []Parameter
	Response    string
	Tag         string
}

type Parameter struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

var endpoints = []Endpoint{
	{
		Name:        "account",
		Method:      "GET",
		Description: "Query account information by address",
		Parameters: []Parameter{
			{Name: "addr", Type: "string", Required: true, Description: "Account address (hex bytes)"},
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
		},
		Response: "QueryResult",
	},
	{
		Name:        "delegatee",
		Method:      "GET",
		Description: "Query delegatee information by address",
		Parameters: []Parameter{
			{Name: "addr", Type: "string", Required: true, Description: "Delegatee address (hex bytes)"},
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
		},
		Response: "QueryResult",
	},
	{
		Name:        "stakes",
		Method:      "GET",
		Description: "Query stakes information by address",
		Parameters: []Parameter{
			{Name: "addr", Type: "string", Required: true, Description: "Address (hex bytes)"},
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
		},
		Response: "QueryResult",
	},
	{
		Name:        "stakes/total_power",
		Method:      "GET",
		Description: "Query total staking power",
		Parameters: []Parameter{
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
		},
		Response: "QueryResult",
	},
	{
		Name:        "stakes/voting_power",
		Method:      "GET",
		Description: "Query total voting power",
		Parameters: []Parameter{
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
		},
		Response: "QueryResult",
	},
	{
		Name:        "reward",
		Method:      "GET",
		Description: "Query reward information by address",
		Parameters: []Parameter{
			{Name: "addr", Type: "string", Required: true, Description: "Address (hex bytes)"},
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
		},
		Response: "QueryResult",
	},
	{
		Name:        "total_supply",
		Method:      "GET",
		Description: "Query total token supply",
		Parameters: []Parameter{
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
		},
		Response: "QueryResult",
	},
	{
		Name:        "total_txfee",
		Method:      "GET",
		Description: "Query total transaction fees",
		Parameters:  []Parameter{},
		Response:    "QueryResult",
	},
	{
		Name:        "proposals",
		Method:      "GET",
		Description: "Query proposal information",
		Parameters: []Parameter{
			{Name: "txhash", Type: "string", Required: true, Description: "Transaction hash (hex bytes)"},
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
		},
		Response: "QueryResult",
	},
	{
		Name:        "proposal",
		Method:      "GET",
		Description: "Query proposal information",
		Parameters: []Parameter{
			{Name: "txhash", Type: "string", Required: true, Description: "Transaction hash (hex bytes)"},
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
		},
		Response: "QueryResult",
	},
	{
		Name:        "rule",
		Method:      "GET",
		Description: "Query governance parameters (deprecated, use gov_params)",
		Parameters: []Parameter{
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
		},
		Response: "QueryResult",
	},
	{
		Name:        "gov_params",
		Method:      "GET",
		Description: "Query governance parameters",
		Parameters: []Parameter{
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
		},
		Response: "QueryResult",
	},
	{
		Name:        "subscribe",
		Method:      "GET",
		Description: "Subscribe to events (WebSocket only)",
		Parameters: []Parameter{
			{Name: "query", Type: "string", Required: true, Description: "Event query string"},
		},
		Response: "ResultSubscribe",
	},
	{
		Name:        "unsubscribe",
		Method:      "GET",
		Description: "Unsubscribe from events (WebSocket only)",
		Parameters: []Parameter{
			{Name: "query", Type: "string", Required: true, Description: "Event query string"},
		},
		Response: "ResultUnsubscribe",
	},
	{
		Name:        "tx_search",
		Method:      "GET",
		Description: "Search transactions",
		Parameters: []Parameter{
			{Name: "query", Type: "string", Required: true, Description: "Search query"},
			{Name: "prove", Type: "boolean", Required: false, Description: "Include proof"},
			{Name: "page", Type: "integer", Required: false, Description: "Page number"},
			{Name: "per_page", Type: "integer", Required: false, Description: "Results per page"},
			{Name: "order_by", Type: "string", Required: false, Description: "Order by (asc/desc)"},
		},
		Response: "ResultTxSearch",
	},
	{
		Name:        "validators",
		Method:      "GET",
		Description: "Query validators",
		Parameters: []Parameter{
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
			{Name: "page", Type: "integer", Required: false, Description: "Page number"},
			{Name: "per_page", Type: "integer", Required: false, Description: "Results per page"},
		},
		Response: "ResultValidators",
	},
	{
		Name:        "vm_call",
		Method:      "GET",
		Description: "Call VM (read-only)",
		Parameters: []Parameter{
			{Name: "addr", Type: "string", Required: true, Description: "Caller address (hex bytes)"},
			{Name: "to", Type: "string", Required: true, Description: "Contract address (hex bytes)"},
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
			{Name: "data", Type: "string", Required: true, Description: "Call data (hex bytes)"},
		},
		Response: "QueryResult",
	},
	{
		Name:        "vm_estimate_gas",
		Method:      "GET",
		Description: "Estimate gas for VM call",
		Parameters: []Parameter{
			{Name: "addr", Type: "string", Required: true, Description: "Caller address (hex bytes)"},
			{Name: "to", Type: "string", Required: true, Description: "Contract address (hex bytes)"},
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
			{Name: "data", Type: "string", Required: true, Description: "Call data (hex bytes)"},
		},
		Response: "QueryResult",
	},
	{
		Name:        "txn",
		Method:      "GET",
		Description: "Query transaction information",
		Parameters:  []Parameter{},
		Response:    "QueryResult",
	},
	// Tendermint Base RPC
	{
		Name:        "health",
		Method:      "GET",
		Description: "Check node health",
		Parameters:  []Parameter{},
		Response:    "ResultHealth",
	},
	{
		Name:        "status",
		Method:      "GET",
		Description: "Get node status",
		Parameters:  []Parameter{},
		Response:    "ResultStatus",
	},
	{
		Name:        "net_info",
		Method:      "GET",
		Description: "Get network info",
		Parameters:  []Parameter{},
		Response:    "ResultNetInfo",
	},
	{
		Name:        "blockchain",
		Method:      "GET",
		Description: "Get blockchain info between min and max height",
		Parameters: []Parameter{
			{Name: "minHeight", Type: "integer", Required: false, Description: "Minimum block height"},
			{Name: "maxHeight", Type: "integer", Required: false, Description: "Maximum block height"},
		},
		Response: "ResultBlockchainInfo",
	},
	{
		Name:        "genesis",
		Method:      "GET",
		Description: "Get genesis file",
		Parameters:  []Parameter{},
		Response:    "ResultGenesis",
	},
	{
		Name:        "genesis_chunked",
		Method:      "GET",
		Description: "Get genesis file in chunks",
		Parameters: []Parameter{
			{Name: "chunk", Type: "integer", Required: true, Description: "Chunk number"},
		},
		Response: "ResultGenesisChunk",
	},
	{
		Name:        "block",
		Method:      "GET",
		Description: "Get block at a given height",
		Parameters: []Parameter{
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
		},
		Response: "ResultBlock",
	},
	{
		Name:        "block_by_hash",
		Method:      "GET",
		Description: "Get block by hash",
		Parameters: []Parameter{
			{Name: "hash", Type: "string", Required: true, Description: "Block hash (hex bytes)"},
		},
		Response: "ResultBlock",
	},
	{
		Name:        "block_results",
		Method:      "GET",
		Description: "Get ABCI results for a block",
		Parameters: []Parameter{
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
		},
		Response: "ResultBlockResults",
	},
	{
		Name:        "commit",
		Method:      "GET",
		Description: "Get block commit at a given height",
		Parameters: []Parameter{
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
		},
		Response: "ResultCommit",
	},
	{
		Name:        "check_tx",
		Method:      "GET",
		Description: "Check transaction validity",
		Parameters: []Parameter{
			{Name: "tx", Type: "string", Required: true, Description: "Transaction bytes (hex encoded)"},
		},
		Response: "ResultCheckTx",
	},
	{
		Name:        "tx",
		Method:      "GET",
		Description: "Get transaction by hash",
		Parameters: []Parameter{
			{Name: "hash", Type: "string", Required: true, Description: "Transaction hash (hex bytes)"},
			{Name: "prove", Type: "boolean", Required: false, Description: "Include proof"},
		},
		Response: "ResultTx",
	},
	{
		Name:        "block_search",
		Method:      "GET",
		Description: "Search for blocks",
		Parameters: []Parameter{
			{Name: "query", Type: "string", Required: true, Description: "Search query"},
			{Name: "page", Type: "integer", Required: false, Description: "Page number"},
			{Name: "per_page", Type: "integer", Required: false, Description: "Results per page"},
			{Name: "order_by", Type: "string", Required: false, Description: "Order by (asc/desc)"},
		},
		Response: "ResultBlockSearch",
	},
	{
		Name:        "consensus_params",
		Method:      "GET",
		Description: "Get consensus parameters",
		Parameters: []Parameter{
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
		},
		Response: "ResultConsensusParams",
	},
	{
		Name:        "consensus_state",
		Method:      "GET",
		Description: "Get consensus state",
		Parameters:  []Parameter{},
		Response:    "ResultConsensusState",
	},
	{
		Name:        "dump_consensus_state",
		Method:      "GET",
		Description: "Dump consensus state",
		Parameters:  []Parameter{},
		Response:    "ResultDumpConsensusState",
	},
	{
		Name:        "unconfirmed_txs",
		Method:      "GET",
		Description: "Get unconfirmed transactions",
		Parameters: []Parameter{
			{Name: "limit", Type: "integer", Required: false, Description: "Maximum number of txs to return"},
		},
		Response: "ResultUnconfirmedTxs",
	},
	{
		Name:        "num_unconfirmed_txs",
		Method:      "GET",
		Description: "Get number of unconfirmed transactions",
		Parameters:  []Parameter{},
		Response:    "ResultNumUnconfirmedTxs",
	},
	{
		Name:        "broadcast_tx_commit",
		Method:      "GET",
		Description: "Broadcast transaction and wait for commit",
		Parameters: []Parameter{
			{Name: "tx", Type: "string", Required: true, Description: "Transaction bytes (hex encoded)"},
		},
		Response: "ResultBroadcastTxCommit",
	},
	{
		Name:        "broadcast_tx_sync",
		Method:      "GET",
		Description: "Broadcast transaction synchronously",
		Parameters: []Parameter{
			{Name: "tx", Type: "string", Required: true, Description: "Transaction bytes (hex encoded)"},
		},
		Response: "ResultBroadcastTx",
	},
	{
		Name:        "broadcast_tx_async",
		Method:      "GET",
		Description: "Broadcast transaction asynchronously",
		Parameters: []Parameter{
			{Name: "tx", Type: "string", Required: true, Description: "Transaction bytes (hex encoded)"},
		},
		Response: "ResultBroadcastTx",
	},
	{
		Name:        "abci_query",
		Method:      "GET",
		Description: "Query the application",
		Parameters: []Parameter{
			{Name: "path", Type: "string", Required: true, Description: "Query path"},
			{Name: "data", Type: "string", Required: false, Description: "Query data (hex bytes)"},
			{Name: "height", Type: "integer", Required: false, Description: "Block height (default: latest)"},
			{Name: "prove", Type: "boolean", Required: false, Description: "Include proof"},
		},
		Response: "ResultABCIQuery",
	},
	{
		Name:        "abci_info",
		Method:      "GET",
		Description: "Get ABCI application info",
		Parameters:  []Parameter{},
		Response:    "ResultABCIInfo",
	},
	{
		Name:        "broadcast_evidence",
		Method:      "GET",
		Description: "Broadcast evidence of misbehavior",
		Parameters: []Parameter{
			{Name: "evidence", Type: "string", Required: true, Description: "Evidence JSON"},
		},
		Response: "ResultBroadcastEvidence",
	},
}

const openapiTemplate = `openapi: 3.0.3
info:
  title: BEATOZ Blockchain API
  description: |
    BEATOZ Blockchain API Documentation

    This API provides access to the BEATOZ blockchain data and functionality.

    **Base URL**: https://rpc-devnet0.beatoz.io

    **Note**: All endpoints are accessed via HTTP GET requests with query parameters.
  version: 1.0.0
  contact:
    name: BEATOZ Team
    url: https://github.com/beatoz/beatoz-go

servers:
  - url: https://rpc-devnet0.beatoz.io
    description: Development Network
  - url: https://rpc-testnet0.beatoz.io
    description: Test Network
  - url: http://localhost:26657
    description: Local development server

paths:
{{- range .Endpoints }}
  /{{ .Name }}:
    {{ .Method | lower }}:
      summary: {{ .Description }}
      operationId: {{ .Name }}
      tags:
        - {{ .Tag }}
{{- if .Parameters }}
      parameters:
{{- range .Parameters }}
        - name: {{ .Name }}
          in: query
          required: {{ .Required }}
          description: "{{ .Description }}"
          schema:
            type: {{ .Type }}
{{- end }}
{{- end }}
      responses:
        '200':
          description: Successful response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/{{ .Response }}'
        '400':
          description: Bad request
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
        '500':
          description: Internal server error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
{{- end }}

components:
  schemas:
    QueryResult:
      type: object
      properties:
        code:
          type: integer
          format: uint32
          description: Response code (0 = success)
        log:
          type: string
          description: Log message
        info:
          type: string
          description: Additional info
        index:
          type: integer
          format: int64
          description: Index
        key:
          type: string
          description: Key (hex bytes)
        value:
          type: object
          description: Result value (JSON)
        proof_ops:
          type: object
          description: Proof operations
        height:
          type: integer
          format: int64
          description: Block height
        codespace:
          type: string
          description: Codespace

    ResultSubscribe:
      type: object
      properties:
        query:
          type: string
          description: Subscribed query

    ResultUnsubscribe:
      type: object
      properties:
        query:
          type: string
          description: Unsubscribed query

    ResultTxSearch:
      type: object
      properties:
        txs:
          type: array
          items:
            type: object
          description: List of transactions
        total_count:
          type: integer
          description: Total number of results

    ResultValidators:
      type: object
      properties:
        block_height:
          type: integer
          format: int64
          description: Block height
        validators:
          type: array
          items:
            type: object
          description: List of validators
        count:
          type: integer
          description: Number of validators
        total:
          type: integer
          description: Total validators

    ResultBlock:
      type: object
      properties:
        block_id:
          type: object
          description: Block ID
        block:
          type: object
          description: Block data

    ResultHealth:
      type: object
      description: Node health response

    ResultStatus:
      type: object
      description: Node status information

    ResultNetInfo:
      type: object
      description: Network information

    ResultBlockchainInfo:
      type: object
      description: Blockchain information

    ResultGenesis:
      type: object
      description: Genesis file

    ResultGenesisChunk:
      type: object
      description: Genesis chunk

    ResultBlockResults:
      type: object
      description: Block results

    ResultCommit:
      type: object
      description: Block commit

    ResultCheckTx:
      type: object
      description: Check transaction result

    ResultTx:
      type: object
      description: Transaction result

    ResultBlockSearch:
      type: object
      description: Block search results

    ResultConsensusParams:
      type: object
      description: Consensus parameters

    ResultConsensusState:
      type: object
      description: Consensus state

    ResultDumpConsensusState:
      type: object
      description: Consensus state dump

    ResultUnconfirmedTxs:
      type: object
      description: Unconfirmed transactions

    ResultNumUnconfirmedTxs:
      type: object
      description: Number of unconfirmed transactions

    ResultBroadcastTxCommit:
      type: object
      description: Broadcast transaction commit result

    ResultBroadcastTx:
      type: object
      description: Broadcast transaction result

    ResultABCIQuery:
      type: object
      description: ABCI query result

    ResultABCIInfo:
      type: object
      description: ABCI info

    ResultBroadcastEvidence:
      type: object
      description: Broadcast evidence result

    Error:
      type: object
      properties:
        code:
          type: integer
          description: Error code
        message:
          type: string
          description: Error message
        data:
          type: string
          description: Additional error data
`

func getTag(name string) string {
	if len(name) >= 3 && name[:3] == "vm_" {
		return "Virtual Machine"
	}
	switch name {
	case "subscribe", "unsubscribe", "tx_search":
		return "WebSocket"
	case "stakes", "stakes/total_power", "stakes/voting_power":
		return "Staking"
	case "gov_params", "rule", "proposal", "proposals":
		return "Governance"
	default:
		return "BEATOZ RPC"
	}
}

func main() {
	// Assign tags to all endpoints
	for i := range endpoints {
		endpoints[i].Tag = getTag(endpoints[i].Name)
	}

	// Group endpoints by tag
	tagMap := make(map[string][]Endpoint)
	for _, ep := range endpoints {
		tagMap[ep.Tag] = append(tagMap[ep.Tag], ep)
	}

	// Sort endpoints within each tag alphabetically
	for tag := range tagMap {
		sort.Slice(tagMap[tag], func(i, j int) bool {
			return tagMap[tag][i].Name < tagMap[tag][j].Name
		})
	}

	// Define tag order
	tagOrder := []string{"BEATOZ RPC", "Governance", "Staking", "Virtual Machine", "WebSocket"}

	// Create sorted endpoint list by tag order, then alphabetically within each tag
	var sortedEndpoints []Endpoint
	for _, tag := range tagOrder {
		if eps, ok := tagMap[tag]; ok {
			sortedEndpoints = append(sortedEndpoints, eps...)
		}
	}

	funcMap := template.FuncMap{
		"lower": func(s string) string {
			return "get"
		},
	}

	tmpl, err := template.New("openapi").Funcs(funcMap).Parse(openapiTemplate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing template: %v\n", err)
		os.Exit(1)
	}

	data := struct {
		Endpoints []Endpoint
	}{
		Endpoints: sortedEndpoints,
	}

	if err := tmpl.Execute(os.Stdout, data); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing template: %v\n", err)
		os.Exit(1)
	}
}
