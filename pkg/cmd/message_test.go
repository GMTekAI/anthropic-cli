package cmd

import (
	"testing"

	"github.com/anthropics/anthropic-cli/v2/internal/mocktest"
	"github.com/anthropics/anthropic-cli/v2/internal/requestflag"
)

func TestMessagesCreate(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"messages", "create",
			"--max-items", "10",
			"--max-tokens", "1024",
			"--message", "{content: [{text: x, type: text, cache_control: {type: ephemeral, ttl: 5m}, citations: [{cited_text: The grass is green. The sky is blue., document_index: 0, document_title: x, end_char_index: 0, start_char_index: 0, type: char_location}]}], role: user}",
			"--model", "claude-opus-5-5",
			"--cache-control", "{type: ephemeral, ttl: 5m}",
			"--container", "{id: id, skills: [{skill_id: pdf, type: anthropic, version: latest}]}",
			"--diagnostics", "{previous_message_id: previous_message_id}",
			"--inference-geo", "inference_geo",
			"--metadata", "{user_id: 13803d75-b4b5-4c3e-b2a2-6f21399b021b}",
			"--output-config", "{effort: low, format: {schema: {foo: bar}, type: json_schema}}",
			"--service-tier", "auto",
			"--speed", "standard",
			"--stop-sequence", "string",
			"--stream=false",
			"--system", "[{text: Today's date is 2024-06-01., type: text, cache_control: {type: ephemeral, ttl: 5m}, citations: [{cited_text: The grass is green. The sky is blue., document_index: 0, document_title: x, end_char_index: 0, start_char_index: 0, type: char_location}]}]",
			"--thinking", "{type: adaptive, display: summarized}",
			"--tool-choice", "{type: auto, disable_parallel_tool_use: true}",
			"--tool", "{input_schema: {type: object, properties: {location: bar, unit: bar}, required: [location]}, name: name, allowed_callers: [direct], cache_control: {type: ephemeral, ttl: 5m}, defer_loading: true, description: Get the current weather in a given location, eager_input_streaming: true, input_examples: [{foo: bar}], strict: true, type: custom}",
			"--user-profile-id", "anthropic-user-profile-id",
			"--workspace-id", "wrkspc_011CZkZaBF1tNoB5wlCeusgy",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(messagesCreate)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"messages", "create",
			"--max-items", "10",
			"--max-tokens", "1024",
			"--message", "{content: [{text: x, type: text, cache_control: {type: ephemeral, ttl: 5m}, citations: [{cited_text: The grass is green. The sky is blue., document_index: 0, document_title: x, end_char_index: 0, start_char_index: 0, type: char_location}]}], role: user}",
			"--model", "claude-opus-5-5",
			"--cache-control.type", "ephemeral",
			"--cache-control.ttl", "5m",
			"--container", "{id: id, skills: [{skill_id: pdf, type: anthropic, version: latest}]}",
			"--diagnostics.previous-message-id", "previous_message_id",
			"--inference-geo", "inference_geo",
			"--metadata.user-id", "13803d75-b4b5-4c3e-b2a2-6f21399b021b",
			"--output-config.effort", "low",
			"--output-config.format", "{schema: {foo: bar}, type: json_schema}",
			"--service-tier", "auto",
			"--speed", "standard",
			"--stop-sequence", "string",
			"--stream=false",
			"--system", "[{text: Today's date is 2024-06-01., type: text, cache_control: {type: ephemeral, ttl: 5m}, citations: [{cited_text: The grass is green. The sky is blue., document_index: 0, document_title: x, end_char_index: 0, start_char_index: 0, type: char_location}]}]",
			"--thinking", "{type: adaptive, display: summarized}",
			"--tool-choice", "{type: auto, disable_parallel_tool_use: true}",
			"--tool", "{input_schema: {type: object, properties: {location: bar, unit: bar}, required: [location]}, name: name, allowed_callers: [direct], cache_control: {type: ephemeral, ttl: 5m}, defer_loading: true, description: Get the current weather in a given location, eager_input_streaming: true, input_examples: [{foo: bar}], strict: true, type: custom}",
			"--user-profile-id", "anthropic-user-profile-id",
			"--workspace-id", "wrkspc_011CZkZaBF1tNoB5wlCeusgy",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"max_tokens: 1024\n" +
			"messages:\n" +
			"  - content:\n" +
			"      - text: x\n" +
			"        type: text\n" +
			"        cache_control:\n" +
			"          type: ephemeral\n" +
			"          ttl: 5m\n" +
			"        citations:\n" +
			"          - cited_text: The grass is green. The sky is blue.\n" +
			"            document_index: 0\n" +
			"            document_title: x\n" +
			"            end_char_index: 0\n" +
			"            start_char_index: 0\n" +
			"            type: char_location\n" +
			"    role: user\n" +
			"model: claude-opus-5-5\n" +
			"cache_control:\n" +
			"  type: ephemeral\n" +
			"  ttl: 5m\n" +
			"container:\n" +
			"  id: id\n" +
			"  skills:\n" +
			"    - skill_id: pdf\n" +
			"      type: anthropic\n" +
			"      version: latest\n" +
			"diagnostics:\n" +
			"  previous_message_id: previous_message_id\n" +
			"inference_geo: inference_geo\n" +
			"metadata:\n" +
			"  user_id: 13803d75-b4b5-4c3e-b2a2-6f21399b021b\n" +
			"output_config:\n" +
			"  effort: low\n" +
			"  format:\n" +
			"    schema:\n" +
			"      foo: bar\n" +
			"    type: json_schema\n" +
			"service_tier: auto\n" +
			"speed: standard\n" +
			"stop_sequences:\n" +
			"  - string\n" +
			"stream: false\n" +
			"system:\n" +
			"  - text: Today's date is 2024-06-01.\n" +
			"    type: text\n" +
			"    cache_control:\n" +
			"      type: ephemeral\n" +
			"      ttl: 5m\n" +
			"    citations:\n" +
			"      - cited_text: The grass is green. The sky is blue.\n" +
			"        document_index: 0\n" +
			"        document_title: x\n" +
			"        end_char_index: 0\n" +
			"        start_char_index: 0\n" +
			"        type: char_location\n" +
			"thinking:\n" +
			"  type: adaptive\n" +
			"  display: summarized\n" +
			"tool_choice:\n" +
			"  type: auto\n" +
			"  disable_parallel_tool_use: true\n" +
			"tools:\n" +
			"  - input_schema:\n" +
			"      type: object\n" +
			"      properties:\n" +
			"        location: bar\n" +
			"        unit: bar\n" +
			"      required:\n" +
			"        - location\n" +
			"    name: name\n" +
			"    allowed_callers:\n" +
			"      - direct\n" +
			"    cache_control:\n" +
			"      type: ephemeral\n" +
			"      ttl: 5m\n" +
			"    defer_loading: true\n" +
			"    description: Get the current weather in a given location\n" +
			"    eager_input_streaming: true\n" +
			"    input_examples:\n" +
			"      - foo: bar\n" +
			"    strict: true\n" +
			"    type: custom\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"messages", "create",
			"--max-items", "10",
			"--user-profile-id", "anthropic-user-profile-id",
			"--workspace-id", "wrkspc_011CZkZaBF1tNoB5wlCeusgy",
		)
	})
}

func TestMessagesCountTokens(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "string")
	t.Run("regular flags", func(t *testing.T) {
		mocktest.TestRunMockTestWithFlags(
			t,
			"messages", "count-tokens",
			"--message", "{content: [{text: x, type: text, cache_control: {type: ephemeral, ttl: 5m}, citations: [{cited_text: The grass is green. The sky is blue., document_index: 0, document_title: x, end_char_index: 0, start_char_index: 0, type: char_location}]}], role: user}",
			"--model", "claude-opus-5-5",
			"--cache-control", "{type: ephemeral, ttl: 5m}",
			"--output-config", "{effort: low, format: {schema: {foo: bar}, type: json_schema}}",
			"--speed", "standard",
			"--system", "[{text: Today's date is 2024-06-01., type: text, cache_control: {type: ephemeral, ttl: 5m}, citations: [{cited_text: The grass is green. The sky is blue., document_index: 0, document_title: x, end_char_index: 0, start_char_index: 0, type: char_location}]}]",
			"--thinking", "{type: adaptive, display: summarized}",
			"--tool-choice", "{type: auto, disable_parallel_tool_use: true}",
			"--tool", "{input_schema: {type: object, properties: {location: bar, unit: bar}, required: [location]}, name: name, allowed_callers: [direct], cache_control: {type: ephemeral, ttl: 5m}, defer_loading: true, description: Get the current weather in a given location, eager_input_streaming: true, input_examples: [{foo: bar}], strict: true, type: custom}",
			"--user-profile-id", "anthropic-user-profile-id",
			"--workspace-id", "wrkspc_011CZkZaBF1tNoB5wlCeusgy",
		)
	})

	t.Run("inner flags", func(t *testing.T) {
		// Check that inner flags have been set up correctly
		requestflag.CheckInnerFlags(messagesCountTokens)

		// Alternative argument passing style using inner flags
		mocktest.TestRunMockTestWithFlags(
			t,
			"messages", "count-tokens",
			"--message", "{content: [{text: x, type: text, cache_control: {type: ephemeral, ttl: 5m}, citations: [{cited_text: The grass is green. The sky is blue., document_index: 0, document_title: x, end_char_index: 0, start_char_index: 0, type: char_location}]}], role: user}",
			"--model", "claude-opus-5-5",
			"--cache-control.type", "ephemeral",
			"--cache-control.ttl", "5m",
			"--output-config.effort", "low",
			"--output-config.format", "{schema: {foo: bar}, type: json_schema}",
			"--speed", "standard",
			"--system", "[{text: Today's date is 2024-06-01., type: text, cache_control: {type: ephemeral, ttl: 5m}, citations: [{cited_text: The grass is green. The sky is blue., document_index: 0, document_title: x, end_char_index: 0, start_char_index: 0, type: char_location}]}]",
			"--thinking", "{type: adaptive, display: summarized}",
			"--tool-choice", "{type: auto, disable_parallel_tool_use: true}",
			"--tool", "{input_schema: {type: object, properties: {location: bar, unit: bar}, required: [location]}, name: name, allowed_callers: [direct], cache_control: {type: ephemeral, ttl: 5m}, defer_loading: true, description: Get the current weather in a given location, eager_input_streaming: true, input_examples: [{foo: bar}], strict: true, type: custom}",
			"--user-profile-id", "anthropic-user-profile-id",
			"--workspace-id", "wrkspc_011CZkZaBF1tNoB5wlCeusgy",
		)
	})

	t.Run("piping data", func(t *testing.T) {
		// Test piping YAML data over stdin
		pipeData := []byte("" +
			"messages:\n" +
			"  - content:\n" +
			"      - text: x\n" +
			"        type: text\n" +
			"        cache_control:\n" +
			"          type: ephemeral\n" +
			"          ttl: 5m\n" +
			"        citations:\n" +
			"          - cited_text: The grass is green. The sky is blue.\n" +
			"            document_index: 0\n" +
			"            document_title: x\n" +
			"            end_char_index: 0\n" +
			"            start_char_index: 0\n" +
			"            type: char_location\n" +
			"    role: user\n" +
			"model: claude-opus-5-5\n" +
			"cache_control:\n" +
			"  type: ephemeral\n" +
			"  ttl: 5m\n" +
			"output_config:\n" +
			"  effort: low\n" +
			"  format:\n" +
			"    schema:\n" +
			"      foo: bar\n" +
			"    type: json_schema\n" +
			"speed: standard\n" +
			"system:\n" +
			"  - text: Today's date is 2024-06-01.\n" +
			"    type: text\n" +
			"    cache_control:\n" +
			"      type: ephemeral\n" +
			"      ttl: 5m\n" +
			"    citations:\n" +
			"      - cited_text: The grass is green. The sky is blue.\n" +
			"        document_index: 0\n" +
			"        document_title: x\n" +
			"        end_char_index: 0\n" +
			"        start_char_index: 0\n" +
			"        type: char_location\n" +
			"thinking:\n" +
			"  type: adaptive\n" +
			"  display: summarized\n" +
			"tool_choice:\n" +
			"  type: auto\n" +
			"  disable_parallel_tool_use: true\n" +
			"tools:\n" +
			"  - input_schema:\n" +
			"      type: object\n" +
			"      properties:\n" +
			"        location: bar\n" +
			"        unit: bar\n" +
			"      required:\n" +
			"        - location\n" +
			"    name: name\n" +
			"    allowed_callers:\n" +
			"      - direct\n" +
			"    cache_control:\n" +
			"      type: ephemeral\n" +
			"      ttl: 5m\n" +
			"    defer_loading: true\n" +
			"    description: Get the current weather in a given location\n" +
			"    eager_input_streaming: true\n" +
			"    input_examples:\n" +
			"      - foo: bar\n" +
			"    strict: true\n" +
			"    type: custom\n")
		mocktest.TestRunMockTestWithPipeAndFlags(
			t, pipeData,
			"messages", "count-tokens",
			"--user-profile-id", "anthropic-user-profile-id",
			"--workspace-id", "wrkspc_011CZkZaBF1tNoB5wlCeusgy",
		)
	})
}
