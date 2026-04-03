package agnogo

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ── Learning Machine ─────────────────────────────────────────────────
//
// Self-improving agents that learn from conversations. The LearningMachine
// coordinates multiple stores that extract, persist, and recall different
// types of knowledge:
//
//   - UserProfile:     structured facts about the user (name, email, preferences)
//   - SessionContext:  what happened in this session (summary, decisions)
//   - EntityMemory:    knowledge about external entities (companies, people)
//
// Usage:
//
//	lm := agnogo.NewLearningMachine(model)
//	lm.AddStore(agnogo.NewUserProfileStore())
//	lm.AddStore(agnogo.NewSessionContextStore())
//
//	agent := agnogo.Agent("...", agnogo.WithLearning(lm))
//
// Before each Run(), the machine recalls relevant memories and injects them
// into the system prompt. After each Run(), it extracts new learnings from
// the conversation and persists them to the session.

// ── LearningStore Interface ──────────────────────────────────────────

// LearningStore is the interface for a learning store. Each store type
// extracts, persists, and recalls a specific kind of knowledge.
type LearningStore interface {
	// Type returns the store type name (e.g., "user_profile", "session_context").
	Type() string

	// Recall retrieves stored knowledge for context injection.
	Recall(ctx context.Context, session *Session) string

	// Process extracts learnings from conversation messages and persists them.
	Process(ctx context.Context, model ModelProvider, session *Session, messages []Message)
}

// ── LearningMachine ──────────────────────────────────────────────────

// LearningMachine coordinates multiple learning stores. It handles context
// building (before agent runs) and learning extraction (after agent runs).
type LearningMachine struct {
	model  ModelProvider
	stores []LearningStore
}

// NewLearningMachine creates a learning machine with a model for LLM-based extraction.
// If model is nil, stores that require LLM extraction will be skipped.
func NewLearningMachine(model ModelProvider) *LearningMachine {
	return &LearningMachine{model: model}
}

// AddStore registers a learning store. Chainable.
func (lm *LearningMachine) AddStore(store LearningStore) *LearningMachine {
	lm.stores = append(lm.stores, store)
	return lm
}

// BuildContext recalls knowledge from all stores and builds a context string
// for injection into the agent's system prompt.
func (lm *LearningMachine) BuildContext(ctx context.Context, session *Session) string {
	if len(lm.stores) == 0 || session == nil {
		return ""
	}

	var parts []string
	for _, store := range lm.stores {
		recalled := store.Recall(ctx, session)
		if recalled != "" {
			parts = append(parts, recalled)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return "LEARNED CONTEXT (from previous interactions):\n" + strings.Join(parts, "\n") + "\n"
}

// Process extracts learnings from conversation messages across all stores.
func (lm *LearningMachine) Process(ctx context.Context, session *Session, messages []Message) {
	if len(lm.stores) == 0 || session == nil {
		return
	}

	for _, store := range lm.stores {
		store.Process(ctx, lm.model, session, messages)
	}
}

// ── UserProfile Store ────────────────────────────────────────────────

// UserProfile holds structured facts about a user.
type UserProfile struct {
	Name            string            `json:"name,omitempty"`
	PreferredName   string            `json:"preferred_name,omitempty"`
	Email           string            `json:"email,omitempty"`
	Phone           string            `json:"phone,omitempty"`
	Company         string            `json:"company,omitempty"`
	Role            string            `json:"role,omitempty"`
	Location        string            `json:"location,omitempty"`
	Language        string            `json:"language,omitempty"`
	Preferences     map[string]string `json:"preferences,omitempty"`
	CustomFields    map[string]string `json:"custom_fields,omitempty"`
	LastUpdated     time.Time         `json:"last_updated,omitempty"`
}

// UserProfileStore extracts and recalls structured user profile data.
type UserProfileStore struct{}

// NewUserProfileStore creates a user profile learning store.
func NewUserProfileStore() *UserProfileStore {
	return &UserProfileStore{}
}

func (s *UserProfileStore) Type() string { return "user_profile" }

func (s *UserProfileStore) Recall(_ context.Context, session *Session) string {
	raw := session.GetStr("_learn_user_profile")
	if raw == "" {
		return ""
	}

	var profile UserProfile
	if err := json.Unmarshal([]byte(raw), &profile); err != nil {
		return ""
	}

	var parts []string
	if profile.Name != "" {
		parts = append(parts, fmt.Sprintf("Name: %s", profile.Name))
	}
	if profile.PreferredName != "" {
		parts = append(parts, fmt.Sprintf("Preferred name: %s", profile.PreferredName))
	}
	if profile.Email != "" {
		parts = append(parts, fmt.Sprintf("Email: %s", profile.Email))
	}
	if profile.Company != "" {
		parts = append(parts, fmt.Sprintf("Company: %s", profile.Company))
	}
	if profile.Role != "" {
		parts = append(parts, fmt.Sprintf("Role: %s", profile.Role))
	}
	if profile.Location != "" {
		parts = append(parts, fmt.Sprintf("Location: %s", profile.Location))
	}
	if profile.Language != "" {
		parts = append(parts, fmt.Sprintf("Language: %s", profile.Language))
	}
	for k, v := range profile.Preferences {
		parts = append(parts, fmt.Sprintf("Preference %s: %s", k, v))
	}
	for k, v := range profile.CustomFields {
		parts = append(parts, fmt.Sprintf("%s: %s", k, v))
	}

	if len(parts) == 0 {
		return ""
	}
	return "[User Profile]\n" + strings.Join(parts, "\n")
}

const userProfileExtractionPrompt = `Extract user profile information from this conversation.
Return a JSON object with ONLY the fields that have new information (omit unknown fields):
{
  "name": "full name",
  "preferred_name": "how they like to be called",
  "email": "email address",
  "phone": "phone number",
  "company": "company/organization",
  "role": "job title/role",
  "location": "city/country",
  "language": "preferred language",
  "preferences": {"key": "value"},
  "custom_fields": {"key": "value"}
}

Return {} if no profile information was shared.`

func (s *UserProfileStore) Process(ctx context.Context, model ModelProvider, session *Session, messages []Message) {
	if model == nil || len(messages) == 0 {
		return
	}

	// Only process recent messages (last 6)
	recent := messages
	if len(recent) > 6 {
		recent = recent[len(recent)-6:]
	}

	extractMsgs := []Message{
		{Role: "system", Content: userProfileExtractionPrompt},
		{Role: "user", Content: formatMessagesForExtraction(recent)},
	}

	resp, err := model.ChatCompletion(ctx, extractMsgs, nil)
	if err != nil {
		slog.Debug("agnogo: user profile extraction failed", "error", err)
		return
	}

	extracted := extractJSON(resp.Text)
	var newProfile UserProfile
	if err := json.Unmarshal([]byte(extracted), &newProfile); err != nil {
		return
	}

	// Merge with existing profile
	existing := loadProfile(session)
	mergeProfile(&existing, &newProfile)
	existing.LastUpdated = time.Now()

	data, err := json.Marshal(existing)
	if err != nil {
		return
	}
	session.Set("_learn_user_profile", string(data))
}

func loadProfile(session *Session) UserProfile {
	raw := session.GetStr("_learn_user_profile")
	if raw == "" {
		return UserProfile{}
	}
	var p UserProfile
	json.Unmarshal([]byte(raw), &p)
	return p
}

func mergeProfile(existing, update *UserProfile) {
	if update.Name != "" {
		existing.Name = update.Name
	}
	if update.PreferredName != "" {
		existing.PreferredName = update.PreferredName
	}
	if update.Email != "" {
		existing.Email = update.Email
	}
	if update.Phone != "" {
		existing.Phone = update.Phone
	}
	if update.Company != "" {
		existing.Company = update.Company
	}
	if update.Role != "" {
		existing.Role = update.Role
	}
	if update.Location != "" {
		existing.Location = update.Location
	}
	if update.Language != "" {
		existing.Language = update.Language
	}
	if len(update.Preferences) > 0 {
		if existing.Preferences == nil {
			existing.Preferences = make(map[string]string)
		}
		for k, v := range update.Preferences {
			existing.Preferences[k] = v
		}
	}
	if len(update.CustomFields) > 0 {
		if existing.CustomFields == nil {
			existing.CustomFields = make(map[string]string)
		}
		for k, v := range update.CustomFields {
			existing.CustomFields[k] = v
		}
	}
}

// ── SessionContext Store ─────────────────────────────────────────────

// SessionContext captures what happened in a session.
type SessionContext struct {
	Summary   string `json:"summary,omitempty"`
	Decisions string `json:"decisions,omitempty"`
	Outcomes  string `json:"outcomes,omitempty"`
	Topics    string `json:"topics,omitempty"`
}

// SessionContextStore extracts and recalls session summaries.
type SessionContextStore struct{}

// NewSessionContextStore creates a session context learning store.
func NewSessionContextStore() *SessionContextStore {
	return &SessionContextStore{}
}

func (s *SessionContextStore) Type() string { return "session_context" }

func (s *SessionContextStore) Recall(_ context.Context, session *Session) string {
	raw := session.GetStr("_learn_session_context")
	if raw == "" {
		return ""
	}

	var sc SessionContext
	if err := json.Unmarshal([]byte(raw), &sc); err != nil {
		return ""
	}

	var parts []string
	if sc.Summary != "" {
		parts = append(parts, "Summary: "+sc.Summary)
	}
	if sc.Decisions != "" {
		parts = append(parts, "Decisions: "+sc.Decisions)
	}
	if sc.Outcomes != "" {
		parts = append(parts, "Outcomes: "+sc.Outcomes)
	}
	if sc.Topics != "" {
		parts = append(parts, "Topics: "+sc.Topics)
	}

	if len(parts) == 0 {
		return ""
	}
	return "[Session Context]\n" + strings.Join(parts, "\n")
}

const sessionContextExtractionPrompt = `Summarize this conversation. Return JSON:
{
  "summary": "brief summary of what was discussed",
  "decisions": "any decisions made",
  "outcomes": "results or actions taken",
  "topics": "main topics covered"
}

Return {} if the conversation is too short to summarize.`

func (s *SessionContextStore) Process(ctx context.Context, model ModelProvider, session *Session, messages []Message) {
	if model == nil || len(messages) < 3 {
		return // need at least a few turns to summarize
	}

	extractMsgs := []Message{
		{Role: "system", Content: sessionContextExtractionPrompt},
		{Role: "user", Content: formatMessagesForExtraction(messages)},
	}

	resp, err := model.ChatCompletion(ctx, extractMsgs, nil)
	if err != nil {
		slog.Debug("agnogo: session context extraction failed", "error", err)
		return
	}

	extracted := extractJSON(resp.Text)
	var sc SessionContext
	if err := json.Unmarshal([]byte(extracted), &sc); err != nil {
		return
	}

	data, err := json.Marshal(sc)
	if err != nil {
		return
	}
	session.Set("_learn_session_context", string(data))
}

// ── EntityMemory Store ───────────────────────────────────────────────

// EntityMemory holds knowledge about an external entity (person, company, project).
type EntityMemory struct {
	EntityID   string   `json:"entity_id"`
	EntityType string   `json:"entity_type"` // "person", "company", "project"
	Facts      []string `json:"facts,omitempty"`
	Events     []string `json:"events,omitempty"`
}

// EntityMemoryStore extracts and recalls facts about entities mentioned in conversation.
type EntityMemoryStore struct{}

// NewEntityMemoryStore creates an entity memory learning store.
func NewEntityMemoryStore() *EntityMemoryStore {
	return &EntityMemoryStore{}
}

func (s *EntityMemoryStore) Type() string { return "entity_memory" }

func (s *EntityMemoryStore) Recall(_ context.Context, session *Session) string {
	raw := session.GetStr("_learn_entities")
	if raw == "" {
		return ""
	}

	var entities []EntityMemory
	if err := json.Unmarshal([]byte(raw), &entities); err != nil {
		return ""
	}

	if len(entities) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("[Known Entities]\n")
	for _, e := range entities {
		sb.WriteString(fmt.Sprintf("%s (%s):\n", e.EntityID, e.EntityType))
		for _, f := range e.Facts {
			sb.WriteString(fmt.Sprintf("  - %s\n", f))
		}
		for _, ev := range e.Events {
			sb.WriteString(fmt.Sprintf("  - [event] %s\n", ev))
		}
	}
	return sb.String()
}

const entityExtractionPrompt = `Extract entities (people, companies, projects) mentioned in this conversation.
Return a JSON array:
[
  {
    "entity_id": "lowercase_identifier",
    "entity_type": "person|company|project",
    "facts": ["timeless fact about entity"],
    "events": ["time-bound event involving entity"]
  }
]

Return [] if no notable entities were discussed.`

func (s *EntityMemoryStore) Process(ctx context.Context, model ModelProvider, session *Session, messages []Message) {
	if model == nil || len(messages) < 2 {
		return
	}

	recent := messages
	if len(recent) > 8 {
		recent = recent[len(recent)-8:]
	}

	extractMsgs := []Message{
		{Role: "system", Content: entityExtractionPrompt},
		{Role: "user", Content: formatMessagesForExtraction(recent)},
	}

	resp, err := model.ChatCompletion(ctx, extractMsgs, nil)
	if err != nil {
		slog.Debug("agnogo: entity extraction failed", "error", err)
		return
	}

	extracted := extractJSON(resp.Text)
	var newEntities []EntityMemory
	if err := json.Unmarshal([]byte(extracted), &newEntities); err != nil {
		return
	}

	if len(newEntities) == 0 {
		return
	}

	// Merge with existing entities
	existing := loadEntities(session)
	merged := mergeEntities(existing, newEntities)

	data, err := json.Marshal(merged)
	if err != nil {
		return
	}
	session.Set("_learn_entities", string(data))
}

func loadEntities(session *Session) []EntityMemory {
	raw := session.GetStr("_learn_entities")
	if raw == "" {
		return nil
	}
	var entities []EntityMemory
	json.Unmarshal([]byte(raw), &entities)
	return entities
}

func mergeEntities(existing, updates []EntityMemory) []EntityMemory {
	byID := make(map[string]*EntityMemory)
	for i := range existing {
		byID[existing[i].EntityID] = &existing[i]
	}

	for _, update := range updates {
		if e, ok := byID[update.EntityID]; ok {
			// Merge facts and events (deduplicate)
			e.Facts = dedup(append(e.Facts, update.Facts...))
			e.Events = dedup(append(e.Events, update.Events...))
			if update.EntityType != "" {
				e.EntityType = update.EntityType
			}
		} else {
			copy := update
			byID[update.EntityID] = &copy
		}
	}

	result := make([]EntityMemory, 0, len(byID))
	for _, e := range byID {
		result = append(result, *e)
	}
	return result
}

func dedup(items []string) []string {
	seen := make(map[string]bool, len(items))
	var result []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// ── Helpers ──────────────────────────────────────────────────────────

// formatMessagesForExtraction formats conversation messages for LLM extraction.
func formatMessagesForExtraction(messages []Message) string {
	var sb strings.Builder
	for _, m := range messages {
		if m.Role == "system" {
			continue
		}
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, m.Content))
	}
	return sb.String()
}

// ── Option ───────────────────────────────────────────────────────────

// WithLearning configures a LearningMachine for the agent.
// The machine's context is injected before each model call,
// and learnings are extracted after each response.
//
//	lm := agnogo.NewLearningMachine(model)
//	lm.AddStore(agnogo.NewUserProfileStore())
//	agent := agnogo.Agent("...", agnogo.WithLearning(lm))
func WithLearning(lm *LearningMachine) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.learning = lm
	})
}
