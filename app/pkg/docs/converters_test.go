package docs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jlewi/cloud-assistant/app/pkg/testutil"
	"github.com/jlewi/cloud-assistant/protos/gen/cassie"
)

func Test_MarkdownToBlocks(t *testing.T) {
	type testCase struct {
		name     string
		inFile   string
		expected []*cassie.Block
	}

	cases := []testCase{
		{
			name:   "simple",
			inFile: "testdoc.md",
			expected: []*cassie.Block{
				{
					Kind:     cassie.BlockKind_MARKUP,
					Metadata: make(map[string]string),
					Contents: "# Section 1",
					Outputs:  []*cassie.BlockOutput{},
				},
				{
					Kind:     cassie.BlockKind_MARKUP,
					Metadata: make(map[string]string),
					Contents: "This is section 1",
					Outputs:  []*cassie.BlockOutput{},
				},
				{
					Kind: cassie.BlockKind_CODE,
					Metadata: map[string]string{
						"runme.dev/name":          "package-main",
						"runme.dev/nameGenerated": "true",
					},
					Language: "go",
					Contents: "package main\n\nfunc main() {\n...\n}",
					Outputs:  []*cassie.BlockOutput{},
				},
				{
					Kind:     cassie.BlockKind_MARKUP,
					Metadata: make(map[string]string),
					Contents: "Breaking text",
					Outputs:  []*cassie.BlockOutput{},
				},
				{
					Kind: cassie.BlockKind_CODE,
					Metadata: map[string]string{
						"runme.dev/name":          "echo-hello",
						"runme.dev/nameGenerated": "true",
					},
					Language: "bash",
					Contents: "echo \"Hello, World!\"",
					Outputs: []*cassie.BlockOutput{
						{
							Items: []*cassie.BlockOutputItem{
								{
									TextData: "hello, world!",
								}},
						},
					},
				},
				{
					Kind:     cassie.BlockKind_MARKUP,
					Metadata: make(map[string]string),
					Contents: "## Subsection",
					Outputs:  []*cassie.BlockOutput{},
				},
			},
		},
		{
			name:   "list-nested",
			inFile: "list.md",
			expected: []*cassie.Block{
				{
					Kind:     cassie.BlockKind_MARKUP,
					Metadata: make(map[string]string),
					Contents: "Test code blocks nested in a list",
					Outputs:  []*cassie.BlockOutput{},
				},
				{
					Kind:     cassie.BlockKind_MARKUP,
					Metadata: make(map[string]string),
					Contents: "1. First command",
					Outputs:  []*cassie.BlockOutput{},
				},
				{
					Kind: cassie.BlockKind_CODE,
					Metadata: map[string]string{
						"runme.dev/name":          "echo-1",
						"runme.dev/nameGenerated": "true",
					},
					Language: "bash",
					Contents: "echo 1",
					Outputs:  []*cassie.BlockOutput{},
				},
				{
					Kind:     cassie.BlockKind_MARKUP,
					Metadata: make(map[string]string),
					Contents: "2. Second command",
					Outputs:  []*cassie.BlockOutput{},
				},
				{
					Kind: cassie.BlockKind_CODE,
					Metadata: map[string]string{
						"runme.dev/name":          "echo-2",
						"runme.dev/nameGenerated": "true",
					},
					Language: "bash",
					Contents: "echo 2",
					Outputs:  []*cassie.BlockOutput{},
				},
			},
		},
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fPath := filepath.Join(cwd, "test_data", c.inFile)
			raw, err := os.ReadFile(fPath)
			if err != nil {
				t.Fatalf("Failed to read raw file: %v", err)
			}
			actual, err := MarkdownToBlocks(string(raw))
			if err != nil {
				t.Fatalf("MarkdownToBlocks(%v) returned error %v", c.inFile, err)
			}
			if len(actual) != len(c.expected) {
				t.Errorf("Expected %v blocks got %v", len(c.expected), len(actual))
			}

			for i, eBlock := range c.expected {
				if i >= len(actual) {
					break
				}

				aBlock := actual[i]

				opts := cmp.Options{
					// ignore Id because it will be unique each time it gets run
					cmpopts.IgnoreFields(cassie.Block{}, "Id"),
				}

				// Zero out the metadata field for id
				delete(aBlock.Metadata, "runme.dev/id")

				if d := cmp.Diff(eBlock, aBlock, testutil.BlockComparer, opts); d != "" {
					t.Errorf("Unexpected diff block %d:\n%s", i, d)
				}
			}
		})
	}
}

func Test_BlockToMarkdown(t *testing.T) {
	type testCase struct {
		name      string
		block     *cassie.Block
		maxLength int
		expected  string
	}

	testCases := []testCase{
		{
			name: "markup",
			block: &cassie.Block{
				Kind:     cassie.BlockKind_MARKUP,
				Contents: "This is a test",
			},
			expected: "This is a test\n",
		},
		{
			name: "code",
			block: &cassie.Block{
				Kind:     cassie.BlockKind_CODE,
				Contents: "echo \"something something\"",
				Outputs: []*cassie.BlockOutput{
					{
						Items: []*cassie.BlockOutputItem{
							{
								TextData: "something something",
							},
						},
					},
				},
			},
			expected: "```bash\necho \"something something\"\n```\n```output\nsomething something\n```\n",
		},
		{
			name: "filter-by-mime-type",
			block: &cassie.Block{
				Kind:     cassie.BlockKind_CODE,
				Contents: "echo \"something something\"",
				Outputs: []*cassie.BlockOutput{
					{
						Items: []*cassie.BlockOutputItem{
							{
								TextData: "Should be excluded",
								Mime:     StatefulRunmeOutputItemsMimeType,
							},
							{
								TextData: "Terminal be excluded",
								Mime:     StatefulRunmeTerminalMimeType,
							},
							{
								TextData: "Should be included",
								Mime:     "application/vnd.code.notebook.stdout",
							},
						},
					},
				},
			},
			expected: "```bash\necho \"something something\"\n```\n```output\nShould be included\n```\n",
		},
		{
			name: "truncate-output",
			block: &cassie.Block{
				Kind:     cassie.BlockKind_CODE,
				Contents: "echo line1\nline2",
				Outputs: []*cassie.BlockOutput{
					{
						Items: []*cassie.BlockOutputItem{
							{
								TextData: "some really long output",
							},
						},
					},
				},
			},
			maxLength: 10,
			expected:  "```bash\n<...code was truncated...>\nline2\n```\n```output\nsome r<...stdout was truncated...>\n```\n",
		},
	}
	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			actual := BlockToMarkdown(c.block, c.maxLength)
			if d := cmp.Diff(c.expected, actual); d != "" {
				t.Errorf("Unexpected diff:\n%s", d)
			}
		})
	}
}
