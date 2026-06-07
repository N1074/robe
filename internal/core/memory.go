package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	MemoryKindPreference      = "preference"
	MemoryKindFact            = "fact"
	MemoryKindDecision        = "decision"
	MemoryKindConstraint      = "constraint"
	MemoryKindTaskContext     = "task_context"
	MemoryKindContactContext  = "contact_context"
	MemoryKindOperationalNote = "operational_note"
)

type MemoryStore interface {
	AddMemory(ctx context.Context, memory Memory) (Memory, error)
	SearchMemories(ctx context.Context, filter MemoryFilter) ([]Memory, error)
	GetMemory(ctx context.Context, id string) (Memory, error)
	ArchiveMemory(ctx context.Context, id string) (Memory, error)
	AddMemoryTag(ctx context.Context, id string, tag string) (Memory, error)
	CreateProject(ctx context.Context, project Project) (Project, error)
	ListProjects(ctx context.Context) ([]Project, error)
	GetProject(ctx context.Context, slug string) (Project, error)
}

type Memory struct {
	ID             string
	Project        ProjectRef
	Kind           string
	Text           string
	Source         string
	Tags           []string
	Confidence     float64
	Importance     int
	Status         string
	Embedding      []float64
	EmbeddingModel string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ExpiresAt      time.Time
}

type Project struct {
	ID          string
	Slug        string
	Name        string
	Description string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ProjectRef struct {
	ID   string
	Slug string
	Name string
}

type MemoryFilter struct {
	Query         string
	ProjectSlug   string
	IncludeGlobal bool
	GlobalOnly    bool
	Kind          string
	Tag           string
	Status        string
	Limit         int
	Semantic      bool
	Embedding     []float64
}

func (a *Assistant) handleRemember(ctx context.Context, text string) (string, error) {
	if a.memory == nil {
		return memoryNotConfiguredMessage(), nil
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return rememberUsage(), nil
	}

	draft, err := parseRememberArgs(text)
	if err != nil {
		return err.Error(), nil
	}

	if draft.Project.Slug == "" {
		draft.Project.Slug = a.activeProject
	}
	draft.Project.Slug = normalizeProjectSlug(draft.Project.Slug)

	memory, err := a.createMemory(ctx, Memory{
		Project:    draft.Project,
		Kind:       normalizeMemoryKind(draft.Kind),
		Text:       draft.Text,
		Source:     "telegram/manual",
		Tags:       draft.Tags,
		Confidence: defaultConfidence(draft.Confidence),
		Importance: defaultImportance(draft.Importance),
		Status:     "active",
	})
	if err != nil {
		if strings.Contains(err.Error(), "exceeds maximum length") || strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "denied") {
			return err.Error(), nil
		}
		return "", err
	}

	return "Memory saved:\n" + formatMemory(memory), nil
}

func (a *Assistant) createMemory(ctx context.Context, memory Memory) (Memory, error) {
	memory.Text = strings.TrimSpace(memory.Text)
	if memory.Text == "" {
		return Memory{}, fmt.Errorf("memory text is required")
	}
	if len(memory.Text) > 1000 {
		return Memory{}, fmt.Errorf("memory text exceeds maximum length of 1000 characters")
	}

	memory.Project.Slug = normalizeProjectSlug(memory.Project.Slug)
	memory.Kind = normalizeMemoryKind(memory.Kind)
	memory.Source = nonEmpty(memory.Source, "telegram/llm")
	memory.Confidence = defaultConfidence(memory.Confidence)
	memory.Importance = defaultImportance(memory.Importance)
	memory.Status = nonEmpty(memory.Status, "active")
	memory.CreatedAt = nonZeroCoreTime(memory.CreatedAt, a.now())
	memory.UpdatedAt = nonZeroCoreTime(memory.UpdatedAt, memory.CreatedAt)

	action := memoryAction(ActionMemoryCreate, memory)
	decision := a.decide(action)
	if decision.Decision == DecisionDeny {
		err := fmt.Errorf("memory creation denied: %s", decision.Reason)
		a.recordAudit(ctx, action, decision, AuditResultRejected, err)
		return Memory{}, err
	}

	if a.embedder != nil {
		embedding, err := a.embedder.Embed(ctx, memory.Text)
		if err == nil {
			memory.Embedding = embedding
			memory.EmbeddingModel = a.embedder.Model()
		}
	}

	stored, err := a.memory.AddMemory(ctx, memory)
	action.ResourceID = stored.ID
	a.recordAudit(ctx, action, decision, AuditResultExecuted, err)
	return stored, err
}

func (a *Assistant) validateMemoryProposal(originalText string, memory Memory) error {
	if a.memory == nil {
		return fmt.Errorf("%s", memoryNotConfiguredMessage())
	}

	if !hasExplicitMemorySignal(originalText) {
		return fmt.Errorf("memory creation requires explicit user intent")
	}
	if strings.TrimSpace(memory.Text) == "" {
		return fmt.Errorf("memory text is required")
	}
	if len(memory.Text) > 1000 {
		return fmt.Errorf("memory text exceeds maximum length of 1000 characters")
	}
	if containsSensitiveMemory(memory.Text) && !hasExplicitMemorySignal(originalText) {
		return fmt.Errorf("sensitive memory requires explicit user intent")
	}
	if !isAllowedMemoryKind(normalizeMemoryKind(memory.Kind)) {
		return fmt.Errorf("unsupported memory kind: %s", memory.Kind)
	}
	return nil
}

func (a *Assistant) handleMemories(ctx context.Context, query string) (string, error) {
	if a.memory == nil {
		return memoryNotConfiguredMessage(), nil
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return memoriesUsage(), nil
	}

	filter, err := parseMemoriesArgs(query)
	if err != nil {
		return err.Error(), nil
	}
	if filter.ProjectSlug == "" {
		filter.ProjectSlug = a.activeProject
	}
	if filter.Status == "" {
		filter.Status = "active"
	}
	if filter.Limit == 0 {
		filter.Limit = 5
	}

	memories, err := a.memory.SearchMemories(ctx, filter)
	if err != nil {
		return "", err
	}

	if len(memories) == 0 {
		return "No memories found.", nil
	}

	var b strings.Builder
	b.WriteString("Memories:\n")
	for _, memory := range memories {
		b.WriteString("- ")
		b.WriteString(formatMemory(memory))
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n"), nil
}

func (a *Assistant) handleAskWithMemory(ctx context.Context, input string) (string, error) {
	if a.llm == nil {
		return "LLM is not configured.", nil
	}
	if a.memory == nil {
		return memoryNotConfiguredMessage(), nil
	}

	memoryQuery, question, ok := strings.Cut(strings.TrimSpace(input), "|")
	if !ok || strings.TrimSpace(memoryQuery) == "" || strings.TrimSpace(question) == "" {
		return "Usage: /askmem <memory query> | <question>", nil
	}

	filter := MemoryFilter{
		Query:         strings.TrimSpace(memoryQuery),
		ProjectSlug:   a.activeProject,
		IncludeGlobal: true,
		Status:        "active",
		Limit:         5,
	}
	if a.embedder != nil {
		embedding, err := a.embedder.Embed(ctx, filter.Query)
		if err != nil {
			return "", err
		}
		filter.Semantic = true
		filter.Embedding = embedding
	}
	memories, err := a.memory.SearchMemories(ctx, filter)
	if err != nil {
		return "", err
	}
	if len(memories) == 0 {
		return "No memories found for that query.", nil
	}

	answer, err := a.llm.Ask(ctx, buildMemoryPrompt(strings.TrimSpace(question), memories))
	if err != nil {
		return "", err
	}

	return "Used memories: " + memoryIDList(memories) + "\n\n" + answer, nil
}

func (a *Assistant) buildPromptWithRelevantMemory(ctx context.Context, prompt string) (string, error) {
	memories, err := a.retrieveRelevantMemories(ctx, prompt)
	if err != nil {
		return "", err
	}
	if len(memories) == 0 {
		return prompt, nil
	}

	var b strings.Builder
	b.WriteString("Relevant memory:\n")
	for _, memory := range memories {
		b.WriteString("- ")
		b.WriteString(formatMemoryForPrompt(memory))
		b.WriteString("\n")
	}
	b.WriteString("\nUser request:\n")
	b.WriteString(prompt)
	return b.String(), nil
}

func (a *Assistant) retrieveRelevantMemories(ctx context.Context, prompt string) ([]Memory, error) {
	if a.memory == nil {
		return nil, nil
	}

	query := strings.TrimSpace(prompt)
	if query == "" {
		return nil, nil
	}

	var embedding []float64
	semantic := false
	if a.embedder != nil {
		var err error
		embedding, err = a.embedder.Embed(ctx, query)
		if err == nil {
			semantic = true
		}
	}

	var out []Memory
	global, err := a.memory.SearchMemories(ctx, MemoryFilter{
		Query:      query,
		GlobalOnly: true,
		Status:     "active",
		Limit:      3,
		Semantic:   semantic,
		Embedding:  embedding,
	})
	if err != nil {
		return nil, err
	}
	out = append(out, global...)

	project := a.projectForText(query)
	if project != "" {
		projectMemories, err := a.memory.SearchMemories(ctx, MemoryFilter{
			Query:       query,
			ProjectSlug: project,
			Status:      "active",
			Limit:       3,
			Semantic:    semantic,
			Embedding:   embedding,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, projectMemories...)
	}

	return uniqueMemories(out, 5), nil
}

func (a *Assistant) projectForText(text string) string {
	if a.activeProject != "" {
		return normalizeProjectSlug(a.activeProject)
	}

	lower := strings.ToLower(text)
	for token, project := range a.projectAliases {
		if token != "" && project != "" && strings.Contains(lower, token) {
			return project
		}
	}
	return ""
}

func formatMemory(memory Memory) string {
	var b strings.Builder
	if strings.TrimSpace(memory.ID) != "" {
		b.WriteString("[")
		b.WriteString(memory.ID)
		b.WriteString("] ")
	}
	b.WriteString("[")
	b.WriteString(normalizeMemoryKind(memory.Kind))
	if memory.Project.Slug != "" {
		b.WriteString("/")
		b.WriteString(memory.Project.Slug)
	}
	b.WriteString("] ")
	b.WriteString(strings.TrimSpace(memory.Text))
	if len(memory.Tags) > 0 {
		b.WriteString(" #")
		b.WriteString(strings.Join(memory.Tags, " #"))
	}
	if !memory.CreatedAt.IsZero() {
		b.WriteString(" (")
		b.WriteString(formatTime(memory.CreatedAt))
		b.WriteString(")")
	}
	return b.String()
}

func buildMemoryPrompt(question string, memories []Memory) string {
	var b strings.Builder
	b.WriteString("Answer the user using only relevant memory when helpful. Do not claim memory contains something it does not contain.\n\nRelevant memory:\n")
	for _, memory := range memories {
		b.WriteString("- ")
		b.WriteString(formatMemory(memory))
		b.WriteString("\n")
	}
	b.WriteString("\nUser question:\n")
	b.WriteString(question)
	return b.String()
}

func formatMemoryForPrompt(memory Memory) string {
	project := memory.Project.Slug
	if project == "" {
		project = "global"
	}
	return "[" + project + "/" + normalizeMemoryKind(memory.Kind) + "/" + importanceLabel(memory.Importance) + "] " + strings.TrimSpace(memory.Text)
}

func memoryIDList(memories []Memory) string {
	ids := make([]string, 0, len(memories))
	for _, memory := range memories {
		if strings.TrimSpace(memory.ID) != "" {
			ids = append(ids, memory.ID)
		}
	}
	if len(ids) == 0 {
		return "unknown"
	}
	return strings.Join(ids, ", ")
}

func memoryNotConfiguredMessage() string {
	return "Memory is not configured yet. Set MEMORY_PROVIDER=postgres and DATABASE_URL, then restart Robe."
}

func (a *Assistant) handleMemory(ctx context.Context, text string) (string, error) {
	if a.memory == nil {
		return memoryNotConfiguredMessage(), nil
	}

	arg := strings.TrimSpace(strings.TrimPrefix(text, "/memory"))
	if arg == "" {
		return memoryUsage(), nil
	}

	switch {
	case strings.HasPrefix(arg, "show "):
		return a.showMemory(ctx, strings.TrimSpace(strings.TrimPrefix(arg, "show ")))

	case strings.HasPrefix(arg, "archive "):
		return a.archiveMemory(ctx, strings.TrimSpace(strings.TrimPrefix(arg, "archive ")))

	case strings.HasPrefix(arg, "tag "):
		return a.tagMemory(ctx, strings.TrimSpace(strings.TrimPrefix(arg, "tag ")))

	default:
		return memoryUsage(), nil
	}
}

func (a *Assistant) showMemory(ctx context.Context, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "Usage: /memory show <id>", nil
	}

	memory, err := a.memory.GetMemory(ctx, id)
	if err != nil {
		return "", err
	}

	return "Memory:\n" + formatMemoryDetail(memory), nil
}

func (a *Assistant) archiveMemory(ctx context.Context, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "Usage: /memory archive <id>", nil
	}

	action := Action{
		Type:         ActionMemoryArchive,
		Actor:        ActorUser,
		Source:       "telegram/manual",
		ResourceType: ResourceMemory,
		ResourceID:   id,
		Summary:      "memory archive",
	}
	decision := a.decide(action)
	if decision.Decision == DecisionDeny {
		err := fmt.Errorf("memory archive denied: %s", decision.Reason)
		a.recordAudit(ctx, action, decision, AuditResultRejected, err)
		return err.Error(), nil
	}

	memory, err := a.memory.ArchiveMemory(ctx, id)
	if err != nil {
		return "", err
	}

	action = memoryAction(ActionMemoryArchive, memory)
	a.recordAudit(ctx, action, decision, AuditResultExecuted, nil)

	return "Memory archived:\n" + formatMemory(memory), nil
}

func (a *Assistant) handleForget(ctx context.Context, id string) (string, error) {
	if a.memory == nil {
		return memoryNotConfiguredMessage(), nil
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "Usage: /forget <memory_id>", nil
	}

	action := Action{
		Type:         ActionMemoryArchive,
		Actor:        ActorUser,
		Source:       "telegram/manual",
		ResourceType: ResourceMemory,
		ResourceID:   id,
		Summary:      "memory archive",
	}
	decision := a.decide(action)
	if decision.Decision == DecisionDeny {
		err := fmt.Errorf("memory archive denied: %s", decision.Reason)
		a.recordAudit(ctx, action, decision, AuditResultRejected, err)
		return err.Error(), nil
	}

	memory, err := a.memory.ArchiveMemory(ctx, id)
	if err != nil {
		return "", err
	}
	action = memoryAction(ActionMemoryArchive, memory)
	a.recordAudit(ctx, action, decision, AuditResultExecuted, nil)
	return "Memory forgotten:\n" + formatMemory(memory), nil
}

func (a *Assistant) tagMemory(ctx context.Context, input string) (string, error) {
	id, tag, ok := strings.Cut(strings.TrimSpace(input), " ")
	if !ok || strings.TrimSpace(id) == "" || strings.TrimSpace(tag) == "" {
		return "Usage: /memory tag <id> <tag>", nil
	}

	id = strings.TrimSpace(id)
	tag = strings.TrimSpace(tag)
	action := Action{
		Type:         ActionMemoryTag,
		Actor:        ActorUser,
		Source:       "telegram/manual",
		ResourceType: ResourceMemory,
		ResourceID:   id,
		Summary:      "memory tag",
		Metadata:     map[string]string{"tag": tag},
	}
	decision := a.decide(action)
	if decision.Decision == DecisionDeny {
		err := fmt.Errorf("memory tag denied: %s", decision.Reason)
		a.recordAudit(ctx, action, decision, AuditResultRejected, err)
		return err.Error(), nil
	}

	memory, err := a.memory.AddMemoryTag(ctx, id, tag)
	if err != nil {
		return "", err
	}

	action = memoryAction(ActionMemoryTag, memory)
	a.recordAudit(ctx, action, decision, AuditResultExecuted, nil)

	return "Memory tagged:\n" + formatMemory(memory), nil
}

func (a *Assistant) handleProject(ctx context.Context, text string) (string, error) {
	if a.memory == nil {
		return memoryNotConfiguredMessage(), nil
	}

	arg := strings.TrimSpace(strings.TrimPrefix(text, "/project"))
	if arg == "" || arg == "status" {
		return a.projectStatus(), nil
	}

	switch {
	case arg == "list":
		return a.listProjects(ctx)

	case strings.HasPrefix(arg, "use "):
		return a.useProject(ctx, strings.TrimSpace(strings.TrimPrefix(arg, "use ")))

	case strings.HasPrefix(arg, "create "):
		return a.createProject(ctx, strings.TrimSpace(strings.TrimPrefix(arg, "create ")))

	default:
		return projectUsage(), nil
	}
}

func (a *Assistant) createProject(ctx context.Context, input string) (string, error) {
	slug, name, ok := strings.Cut(input, "|")
	if !ok {
		slug = input
		name = input
	}

	project, err := a.memory.CreateProject(ctx, Project{
		Slug:   strings.TrimSpace(slug),
		Name:   strings.TrimSpace(name),
		Status: "active",
	})
	if err != nil {
		return "", err
	}

	return "Project created:\n" + formatProject(project), nil
}

func (a *Assistant) useProject(ctx context.Context, slug string) (string, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "Usage: /project use <slug>", nil
	}

	project, err := a.memory.GetProject(ctx, slug)
	if err != nil {
		return "", err
	}

	a.activeProject = project.Slug
	return "Active project: " + formatProject(project), nil
}

func (a *Assistant) listProjects(ctx context.Context) (string, error) {
	projects, err := a.memory.ListProjects(ctx)
	if err != nil {
		return "", err
	}
	if len(projects) == 0 {
		return "No projects found.", nil
	}

	var b strings.Builder
	b.WriteString("Projects:\n")
	for _, project := range projects {
		b.WriteString("- ")
		b.WriteString(formatProject(project))
		if project.Slug == a.activeProject {
			b.WriteString(" (active)")
		}
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n"), nil
}

func (a *Assistant) projectStatus() string {
	if a.activeProject == "" {
		return "Active project: global"
	}
	return "Active project: " + a.activeProject
}

func formatProject(project Project) string {
	return project.Slug + " | " + project.Name
}

func formatMemoryDetail(memory Memory) string {
	var b strings.Builder
	b.WriteString(formatMemory(memory))
	b.WriteString("\nSource: ")
	b.WriteString(nonEmpty(memory.Source, "unknown"))
	b.WriteString("\nStatus: ")
	b.WriteString(nonEmpty(memory.Status, "active"))
	b.WriteString("\nImportance: ")
	b.WriteString(fmt.Sprintf("%d", defaultImportance(memory.Importance)))
	b.WriteString("\nConfidence: ")
	b.WriteString(fmt.Sprintf("%.2f", defaultConfidence(memory.Confidence)))
	if !memory.UpdatedAt.IsZero() {
		b.WriteString("\nUpdated: ")
		b.WriteString(formatTime(memory.UpdatedAt))
	}
	if !memory.ExpiresAt.IsZero() {
		b.WriteString("\nExpires: ")
		b.WriteString(formatTime(memory.ExpiresAt))
	}
	return b.String()
}

func parseRememberArgs(input string) (Memory, error) {
	var memory Memory
	memory.Kind = "note"
	memory.Status = "active"
	memory.Importance = 3
	memory.Confidence = 1.0

	rest := strings.TrimSpace(input)
	for strings.HasPrefix(rest, "--") {
		flag, tail, ok := strings.Cut(rest, " ")
		if !ok {
			return Memory{}, fmt.Errorf("missing value for %s", flag)
		}
		value, remaining, _ := strings.Cut(strings.TrimSpace(tail), " ")
		if strings.TrimSpace(value) == "" {
			return Memory{}, fmt.Errorf("missing value for %s", flag)
		}

		switch flag {
		case "--project":
			memory.Project.Slug = strings.TrimSpace(value)
		case "--kind":
			memory.Kind = strings.TrimSpace(value)
		case "--tags":
			memory.Tags = splitCSV(value)
		case "--importance":
			parsed, err := parseInt(value)
			if err != nil {
				return Memory{}, fmt.Errorf("invalid importance: %w", err)
			}
			memory.Importance = parsed
		default:
			return Memory{}, fmt.Errorf("unknown remember flag: %s", flag)
		}

		rest = strings.TrimSpace(remaining)
	}

	memory.Text = rest
	if memory.Text == "" {
		return Memory{}, fmt.Errorf("memory text is required")
	}

	return memory, nil
}

func parseMemoriesArgs(input string) (MemoryFilter, error) {
	var filter MemoryFilter

	rest := strings.TrimSpace(input)
	for strings.HasPrefix(rest, "--") {
		flag, tail, ok := strings.Cut(rest, " ")
		if !ok {
			return MemoryFilter{}, fmt.Errorf("missing value for %s", flag)
		}
		value, remaining, _ := strings.Cut(strings.TrimSpace(tail), " ")
		if strings.TrimSpace(value) == "" {
			return MemoryFilter{}, fmt.Errorf("missing value for %s", flag)
		}

		switch flag {
		case "--project":
			filter.ProjectSlug = strings.TrimSpace(value)
		case "--kind":
			filter.Kind = strings.TrimSpace(value)
		case "--tag":
			filter.Tag = strings.TrimSpace(value)
		default:
			return MemoryFilter{}, fmt.Errorf("unknown memories flag: %s", flag)
		}

		rest = strings.TrimSpace(remaining)
	}

	filter.Query = rest
	if filter.Query == "" {
		return MemoryFilter{}, fmt.Errorf("memory query is required")
	}

	return filter, nil
}

func rememberUsage() string {
	return "Usage: /remember [--project slug] [--kind preference|fact|decision|constraint|task_context|contact_context|operational_note] [--tags a,b] <text>"
}

func memoriesUsage() string {
	return "Usage: /memories [--project slug] [--kind kind] [--tag tag] <query>"
}

func projectUsage() string {
	return "Project commands:\n/project list\n/project create <slug> | <name>\n/project use <slug>\n/project status"
}

func memoryUsage() string {
	return "Memory commands:\n/memory show <id>\n/memory archive <id>\n/memory tag <id> <tag>"
}

func splitCSV(value string) []string {
	raw := strings.Split(value, ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func parseInt(value string) (int, error) {
	var out int
	_, err := fmt.Sscanf(value, "%d", &out)
	return out, err
}

func defaultImportance(value int) int {
	if value <= 0 {
		return 3
	}
	return value
}

func defaultConfidence(value float64) float64 {
	if value <= 0 {
		return 1.0
	}
	return value
}

func normalizeProjectSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	if value == "global" {
		return ""
	}
	return value
}

func normalizeProjectAliases(aliases map[string]string) map[string]string {
	out := make(map[string]string, len(aliases)+1)
	out[""] = ""
	out["global"] = ""
	for alias, project := range aliases {
		alias = normalizeProjectSlug(alias)
		project = normalizeProjectSlug(project)
		if alias == "" || project == "" {
			continue
		}
		out[alias] = project
		out[project] = project
	}
	return out
}

func projectHintList(aliases map[string]string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, project := range aliases {
		if project == "" || seen[project] {
			continue
		}
		seen[project] = true
		out = append(out, project)
	}
	sort.Strings(out)
	return out
}

func normalizeMemoryKind(value string) string {
	switch strings.TrimSpace(value) {
	case "", "note", "project_knowledge":
		return MemoryKindFact
	case "operational":
		return MemoryKindOperationalNote
	case "task":
		return MemoryKindTaskContext
	case "contact":
		return MemoryKindContactContext
	default:
		return strings.TrimSpace(value)
	}
}

func isAllowedMemoryKind(kind string) bool {
	switch kind {
	case MemoryKindPreference, MemoryKindFact, MemoryKindDecision, MemoryKindConstraint, MemoryKindTaskContext, MemoryKindContactContext, MemoryKindOperationalNote:
		return true
	default:
		return false
	}
}

func hasExplicitMemorySignal(text string) bool {
	text = strings.ToLower(text)
	signals := []string{
		"remember that",
		"remember this",
		"recuerda que",
		"recuerda esto",
		"ten en cuenta que",
		"from now on",
		"de ahora en adelante",
		"a partir de ahora",
	}
	for _, signal := range signals {
		if strings.Contains(text, signal) {
			return true
		}
	}
	return false
}

func containsSensitiveMemory(text string) bool {
	text = strings.ToLower(text)
	signals := []string{
		"password",
		"contraseña",
		"token",
		"api key",
		"apikey",
		"secret",
		"secreto",
		"credential",
		"credencial",
		"ssn",
		"dni",
		"credit card",
		"tarjeta",
		"bank account",
		"cuenta bancaria",
	}
	for _, signal := range signals {
		if strings.Contains(text, signal) {
			return true
		}
	}
	return false
}

func importanceLabel(value int) string {
	switch {
	case value >= 4:
		return "high"
	case value <= 2:
		return "low"
	default:
		return "medium"
	}
}

func uniqueMemories(memories []Memory, limit int) []Memory {
	if limit <= 0 {
		limit = len(memories)
	}

	seen := make(map[string]bool)
	out := make([]Memory, 0, len(memories))
	for _, memory := range memories {
		key := memory.ID
		if key == "" {
			key = memory.Project.Slug + "\x00" + memory.Kind + "\x00" + memory.Text
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, memory)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func nonZeroCoreTime(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}
