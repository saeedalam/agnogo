//go:build ignore

// Research Assistant — find papers, extract insights, generate reports.
//
//	source ../../.env && go run main.go
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

func main() {
	if os.Getenv("OPENAI_API_KEY") == "" {
		fmt.Println("Run: source ../../.env && go run main.go")
		os.Exit(1)
	}
	setup()

	reader := agnogo.Agent(
		`You are a research paper analyst for a Go AI agent framework developer.
For each paper, extract 2-5 techniques with IMPLEMENTATION DETAILS — not summaries.

For each technique you MUST include:
- The algorithm or pseudocode (step by step, what the code does)
- Input: what data it needs (e.g. "the LLM response text", "3 sample responses")
- Output: what it produces (e.g. "a float64 confidence score 0-1")
- Key formula or logic (e.g. "entropy = -sum(p * log(p))")
- Example: a concrete input/output example
- Go implementation hint: which Go types, patterns, or stdlib packages to use

Do NOT write vague descriptions like "uses Bayesian estimation to model distributions."
DO write: "1. Generate N responses to same prompt. 2. Group by meaning (if responses say the same thing differently, they're one cluster). 3. Entropy = -sum(p_i * log(p_i)) where p_i = cluster_size/N. 4. If entropy > threshold → hallucination."`,
		agnogo.Tools(saveTechniqueTool()),
		agnogo.WithOpenAI("gpt-4.1-mini"),
		agnogo.WithMaxLoops(12),
	)

	reporter := agnogo.Agent(
		`You write implementation-focused research reports for a Go developer.
For each technique:
1. One-line summary
2. The algorithm (numbered steps)
3. Go implementation sketch (types, function signature, key logic)
4. Difficulty rating and estimated lines of code
5. What existing code to modify (if improving an agent framework)

Be concrete. "Add function X to file Y" not "consider implementing Z."`,
		agnogo.WithOpenAI("gpt-4.1-mini"),
	)

	ctx := context.Background()
	scan := bufio.NewScanner(os.Stdin)
	var papers []Paper

	fmt.Println("Research Assistant")
	fmt.Println("─────────────────")
	fmt.Println("  search <topic>    find papers")
	fmt.Println("  <numbers>         pick, read, and report (e.g. 1,3,5)")
	fmt.Println("  knowledge         show all techniques")
	fmt.Println("  report            regenerate report from all techniques")
	fmt.Println("  quit")

	for {
		fmt.Print("\n> ")
		if !scan.Scan() {
			return
		}
		input := strings.TrimSpace(scan.Text())
		if input == "" {
			continue
		}

		// Bare numbers → select + read + report in one step
		if _, err := strconv.Atoi(strings.Split(input, ",")[0]); err == nil {
			selected := selectPapers(papers, input)
			if len(selected) > 0 {
				readPapers(ctx, reader, selected)
				generateReport(ctx, reporter)
			}
			continue
		}

		cmd, arg := splitCmd(input)
		switch cmd {
		case "search":
			papers = searchPapers(ctx, arg)
		case "select":
			selected := selectPapers(papers, arg)
			if len(selected) > 0 {
				readPapers(ctx, reader, selected)
				generateReport(ctx, reporter)
			}
		case "knowledge":
			showKnowledge()
		case "report":
			generateReport(ctx, reporter)
		case "quit", "exit", "q":
			return
		default:
			fmt.Println("try: search <topic>")
		}
	}
}

// ─── Search ─────────────────────────────────────────────

func searchPapers(ctx context.Context, topic string) []Paper {
	if topic == "" {
		fmt.Println("usage: search <topic>")
		return nil
	}
	fmt.Printf("searching: %s\n\n", topic)

	papers, err := arxivSearch(ctx, topic, 10)
	if err != nil {
		fmt.Println("error:", err)
		return nil
	}

	for i, p := range papers {
		mark := " "
		if done(p.ID) {
			mark = "*"
		}
		fmt.Printf(" %s %2d. %s\n", mark, i+1, short(p.Title, 75))
	}
	fmt.Println("\n(* already read)  type numbers: 1,3,5")
	return papers
}

// ─── Select ─────────────────────────────────────────────

func selectPapers(papers []Paper, indices string) []Paper {
	if len(papers) == 0 || indices == "" {
		fmt.Println("search first")
		return nil
	}

	var selected []Paper
	for _, s := range strings.Split(indices, ",") {
		i, _ := strconv.Atoi(strings.TrimSpace(s))
		if i < 1 || i > len(papers) {
			continue
		}
		p := papers[i-1]
		if done(p.ID) {
			fmt.Printf("  skip (already read): %s\n", short(p.Title, 50))
			continue
		}
		selected = append(selected, p)
	}

	if len(selected) == 0 {
		fmt.Println("no new papers selected")
	}
	return selected
}

// ─── Read ───────────────────────────────────────────────

func readPapers(ctx context.Context, agent *agnogo.Core, papers []Paper) {
	total := len(papers)
	fmt.Printf("\nReading %d papers:\n", total)

	for i, p := range papers {
		fmt.Printf("\n  [%d/%d] %s\n", i+1, total, short(p.Title, 55))
		fmt.Printf("         ")

		session := agnogo.NewSession("read-" + slug(p.Title))
		_, err := agent.Run(ctx, session, fmt.Sprintf(
			"Title: %s\nAuthors: %s\n\nAbstract:\n%s\n\nExtract techniques.",
			p.Title, p.Authors, p.Summary))
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}
		markDone(p.ID)
	}
	fmt.Println()
}

// ─── Report ─────────────────────────────────────────────

func generateReport(ctx context.Context, reporter *agnogo.Core) {
	techniques := loadAll[Technique]("data/knowledge")
	if len(techniques) == 0 {
		return
	}

	fmt.Printf("Generating report from %d techniques...\n", len(techniques))

	var buf strings.Builder
	for _, t := range techniques {
		buf.WriteString(fmt.Sprintf("## %s [%s] (%s)\n%s\nAlgorithm: %s\nInput: %s\nOutput: %s\nGo: %s\nSource: %s\n\n",
			t.Name, t.Category, t.Difficulty, t.Description,
			t.Algorithm, t.Input, t.Output, t.GoHint, t.Source))
	}

	text, err := reporter.Ask(ctx, "Write a research report:\n\n"+buf.String())
	if err != nil {
		fmt.Println("report error:", err)
		return
	}

	// Save report
	file := fmt.Sprintf("data/reports/report_%s.md", time.Now().Format("2006-01-02_15-04"))
	os.WriteFile(file, []byte(text), 0644)

	// Show first few lines + link
	lines := strings.Split(text, "\n")
	preview := 8
	if len(lines) < preview {
		preview = len(lines)
	}
	fmt.Println()
	for _, line := range lines[:preview] {
		fmt.Println(line)
	}
	if len(lines) > preview {
		fmt.Printf("\n  ... (%d more lines)\n", len(lines)-preview)
	}
	fmt.Printf("\nFull report: %s\n", file)
}

// ─── Knowledge ──────────────────────────────────────────

func showKnowledge() {
	items := loadAll[Technique]("data/knowledge")
	if len(items) == 0 {
		fmt.Println("empty — search and select papers first")
		return
	}
	fmt.Printf("%d techniques:\n\n", len(items))
	for i, t := range items {
		fmt.Printf("  %d. [%s] %s (%s)\n", i+1, t.Category, t.Name, t.Difficulty)
		fmt.Printf("     %s\n", t.Description)
		fmt.Printf("     Input:  %s\n", short(t.Input, 70))
		fmt.Printf("     Output: %s\n", short(t.Output, 70))
		fmt.Printf("     Algorithm: %s\n", short(t.Algorithm, 100))
		fmt.Printf("     Go: %s\n", short(t.GoHint, 80))
		fmt.Println()
	}
}

// ─── Tool ───────────────────────────────────────────────

func saveTechniqueTool() agnogo.ToolDef {
	return agnogo.TypedTool(
		"save_technique",
		"Save a technique extracted from a paper",
		func(ctx context.Context, in Technique) (string, error) {
			in.Timestamp = time.Now().Format(time.RFC3339)
			saveJSON("data/knowledge", slug(in.Name), in)
			fmt.Printf("[%s] %s\n", in.Category, in.Name)
			return "saved", nil
		},
	)
}

// ─── Types ──────────────────────────────────────────────

type Paper struct {
	Title   string `json:"title"`
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Authors string `json:"authors"`
	PDF     string `json:"pdf_url"`
}

type Technique struct {
	Name        string `json:"name" desc:"Technique name" required:"true"`
	Description string `json:"description" desc:"One-line summary" required:"true"`
	Algorithm   string `json:"algorithm" desc:"Step-by-step algorithm or pseudocode" required:"true"`
	Input       string `json:"input" desc:"What data it needs (e.g. 'LLM response text', '3 sample responses')" required:"true"`
	Output      string `json:"output" desc:"What it produces (e.g. 'confidence score 0-1', 'bool is_hallucination')" required:"true"`
	GoHint      string `json:"go_hint" desc:"Go implementation hint: types, function signature, stdlib packages" required:"true"`
	Source      string `json:"source" desc:"Paper title" required:"true"`
	Category    string `json:"category" desc:"hallucination/reliability/cost/safety/evaluation/performance" required:"true"`
	Difficulty  string `json:"difficulty" desc:"easy/medium/hard" required:"true"`
	Timestamp   string `json:"timestamp,omitempty"`
}

// ─── ArXiv ──────────────────────────────────────────────

func arxivSearch(ctx context.Context, query string, max int) ([]Paper, error) {
	url := fmt.Sprintf(
		"https://export.arxiv.org/api/query?search_query=all:%s&max_results=%d&sortBy=relevance&sortOrder=descending",
		strings.ReplaceAll(query, " ", "+"), max)
	body, err := fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	return parseAtom(body), nil
}

func parseAtom(xml string) []Paper {
	var out []Paper
	for _, e := range between(xml, "<entry>", "</entry>") {
		p := Paper{
			Title:   clean(tag(e, "title")),
			ID:      clean(tag(e, "id")),
			Summary: clean(tag(e, "summary")),
		}
		var names []string
		for _, a := range between(e, "<author>", "</author>") {
			if n := tag(a, "name"); n != "" {
				names = append(names, n)
			}
		}
		p.Authors = strings.Join(names, ", ")
		for _, l := range between(e, "<link", "/>") {
			if strings.Contains(l, `title="pdf"`) {
				p.PDF = attr(l, "href")
			}
		}
		if p.Title != "" {
			out = append(out, p)
		}
	}
	return out
}

// ─── Storage ────────────────────────────────────────────

func setup() {
	for _, d := range []string{"data/papers/.read", "data/knowledge", "data/reports"} {
		os.MkdirAll(d, 0755)
	}
}

func done(id string) bool {
	_, err := os.Stat(filepath.Join("data/papers/.read", slug(id)+".done"))
	return err == nil
}

func markDone(id string) {
	os.WriteFile(filepath.Join("data/papers/.read", slug(id)+".done"), nil, 0644)
}

func saveJSON(dir, name string, v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	os.WriteFile(filepath.Join(dir, name+".json"), data, 0644)
}

func loadAll[T any](dir string) []T {
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	var out []T
	for _, f := range files {
		data, _ := os.ReadFile(f)
		var v T
		if json.Unmarshal(data, &v) == nil {
			out = append(out, v)
		}
	}
	return out
}

// ─── Helpers ────────────────────────────────────────────

func splitCmd(s string) (string, string) {
	if i := strings.IndexByte(s, ' '); i > 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

func slug(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, s)
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	s = strings.Trim(s, "_")
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}

func short(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func fetch(ctx context.Context, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return string(data), err
}

func between(s, open, close string) []string {
	var out []string
	for {
		i := strings.Index(s, open)
		if i < 0 {
			return out
		}
		s = s[i+len(open):]
		j := strings.Index(s, close)
		if j < 0 {
			return out
		}
		out = append(out, s[:j])
		s = s[j+len(close):]
	}
}

func tag(s, name string) string {
	i := strings.Index(s, "<"+name)
	if i < 0 {
		return ""
	}
	s = s[i:]
	if j := strings.IndexByte(s, '>'); j >= 0 {
		s = s[j+1:]
	}
	if k := strings.Index(s, "</"+name+">"); k >= 0 {
		return s[:k]
	}
	return ""
}

func attr(s, name string) string {
	key := name + `="`
	if i := strings.Index(s, key); i >= 0 {
		s = s[i+len(key):]
		if j := strings.IndexByte(s, '"'); j >= 0 {
			return s[:j]
		}
	}
	return ""
}

func clean(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}
