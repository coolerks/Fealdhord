package appagent

import (
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"strings"
	"time"
)

type UserQuestionOption struct {
	Label       string `json:"label" jsonschema:"required,description=选项显示名称"`
	Description string `json:"description,omitempty" jsonschema:"description=可选的简短说明"`
}

type UserQuestionItem struct {
	ID          string               `json:"id,omitempty" jsonschema:"description=问题标识，可省略"`
	Header      string               `json:"header,omitempty" jsonschema:"description=简短分类标签（如 范围、格式）"`
	Question    string               `json:"question" jsonschema:"required,description=要向用户提出的具体问题"`
	MultiSelect bool                 `json:"multi_select,omitempty" jsonschema:"description=是否允许用户多选，默认 false 为单选"`
	Options     []UserQuestionOption `json:"options" jsonschema:"required,description=提供给用户的候选项（1 到 6 项）；界面会自动附带补充输入框"`
}

type AskUserQuestionInput struct {
	Questions []UserQuestionItem `json:"questions" jsonschema:"required,description=要向用户提出的问题列表（1 到 4 个问题）"`
}

type UserQuestionAnswer struct {
	Question string   `json:"question"`
	Selected []string `json:"selected,omitempty"`
	Note     string   `json:"note,omitempty"`
}

type AskUserQuestionOutput struct {
	Answers []UserQuestionAnswer `json:"answers,omitempty"`
	Skipped bool                 `json:"skipped,omitempty"`
}

type PendingQuestion struct {
	ID             string             `json:"id"`
	ConversationID string             `json:"conversationId"`
	Questions      []UserQuestionItem `json:"questions"`
}

type questionReply struct {
	answers   []UserQuestionAnswer
	cancelled bool
}

type questionWait struct {
	item  PendingQuestion
	reply chan questionReply
}

type conversationContextKey struct{}

func withConversationID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, conversationContextKey{}, id)
}

func conversationIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(conversationContextKey{}).(string)
	return id
}

func (r *Registry) PendingQuestions() []PendingQuestion {
	r.mu.Lock()
	defer r.mu.Unlock()
	pending := make([]PendingQuestion, 0, len(r.questions))
	for _, wait := range r.questions {
		pending = append(pending, wait.item)
	}
	slices.SortFunc(pending, func(a, b PendingQuestion) int { return strings.Compare(a.ID, b.ID) })
	return pending
}

func (r *Registry) AnswerQuestion(id string, answers []UserQuestionAnswer, cancelled bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	wait, ok := r.questions[id]
	if ok {
		delete(r.questions, id)
		wait.reply <- questionReply{answers: answers, cancelled: cancelled}
	}
	return ok
}

func (r *Registry) askUser(ctx context.Context, conversationID string, input AskUserQuestionInput) (AskUserQuestionOutput, error) {
	if conversationID == "" {
		return AskUserQuestionOutput{}, errors.New("InteractiveSessionRequired")
	}
	if len(input.Questions) == 0 || len(input.Questions) > 4 {
		return AskUserQuestionOutput{}, errors.New("ValidationError")
	}
	normalized := make([]UserQuestionItem, 0, len(input.Questions))
	for _, q := range input.Questions {
		questionText := strings.TrimSpace(q.Question)
		if questionText == "" || len(q.Options) == 0 || len(q.Options) > 6 {
			return AskUserQuestionOutput{}, errors.New("ValidationError")
		}
		options := make([]UserQuestionOption, 0, len(q.Options))
		for _, opt := range q.Options {
			label := strings.TrimSpace(opt.Label)
			if label == "" {
				return AskUserQuestionOutput{}, errors.New("ValidationError")
			}
			options = append(options, UserQuestionOption{
				Label:       label,
				Description: strings.TrimSpace(opt.Description),
			})
		}
		normalized = append(normalized, UserQuestionItem{
			ID:          strings.TrimSpace(q.ID),
			Header:      strings.TrimSpace(q.Header),
			Question:    questionText,
			MultiSelect: q.MultiSelect,
			Options:     options,
		})
	}

	id := rand.Text()
	item := PendingQuestion{
		ID:             id,
		ConversationID: conversationID,
		Questions:      normalized,
	}
	r.mu.Lock()
	if r.questions == nil {
		r.questions = make(map[string]questionWait)
	}
	if len(r.questions) >= 16 {
		r.mu.Unlock()
		return AskUserQuestionOutput{}, errors.New("QuestionLimitExceeded")
	}
	reply := make(chan questionReply, 1)
	r.questions[id] = questionWait{item: item, reply: reply}
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.questions, id)
		r.mu.Unlock()
	}()

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	select {
	case response := <-reply:
		if err := ctx.Err(); err != nil {
			return AskUserQuestionOutput{}, err
		}
		if response.cancelled {
			return AskUserQuestionOutput{Skipped: true}, nil
		}
		return AskUserQuestionOutput{Answers: response.answers}, nil
	case <-waitCtx.Done():
		return AskUserQuestionOutput{}, waitCtx.Err()
	}
}

func registerAskUserQuestionTool(r *Registry) {
	Register(r, "AskUserQuestion", "向用户提出 1 到 4 个澄清问题以确认偏好或需求，支持单选（multi_select=false）与多选（multi_select=true），界面会自动提供补充输入框。", false, func(ctx context.Context, input AskUserQuestionInput) any {
		result, err := r.askUser(ctx, conversationIDFromContext(ctx), input)
		if err != nil {
			return map[string]string{"error": err.Error()}
		}
		return result
	})
}
